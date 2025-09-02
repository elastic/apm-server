// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package sampling_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub/pubsubtest"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestProcessUnsampled(t *testing.T) {
	cfg, _ := newTempdirConfig(t)
	processor, err := sampling.NewProcessor(cfg, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	in := modelpb.Batch{{
		Trace: &modelpb.Trace{
			Id: "0102030405060708090a0b0c0d0e0f10",
		},
		Transaction: &modelpb.Transaction{
			Type:    "type",
			Id:      "0102030405060708",
			Sampled: false,
		},
	}}
	out := in[:]
	err = processor.ProcessBatch(context.Background(), &out)
	require.NoError(t, err)

	// Unsampled transaction should be reported immediately.
	assert.Equal(t, in, out)
}

func TestProcessAlreadyTailSampled(t *testing.T) {
	config, metricreader := newTempdirConfig(t)

	// Seed event storage with a tail-sampling decisions, to show that
	// subsequent events in the trace will be reported immediately.
	trace1 := modelpb.Trace{Id: "0102030405060708090a0b0c0d0e0f10"}
	trace2 := modelpb.Trace{Id: "0102030405060708090a0b0c0d0e0f11"}
	writer := config.DB.NewBypassReadWriter()
	wOpts := eventstorage.WriterOpts{
		TTL:                 time.Minute,
		StorageLimitInBytes: 0,
	}
	assert.NoError(t, writer.WriteTraceSampled(trace1.Id, true, wOpts))
	assert.NoError(t, writer.Flush())
	writer.Close()

	wOpts.TTL = -1 // expire immediately
	writer = config.DB.NewBypassReadWriter()
	assert.NoError(t, writer.WriteTraceSampled(trace2.Id, true, wOpts))
	assert.NoError(t, writer.Flush())
	writer.Close()

	// Badger transactions created globally before committing the above writes
	// will not see them due to SSI (Serializable Snapshot Isolation). Flush
	// the storage so that new transactions are created for the underlying
	// writer shards that can list all the events committed so far.
	require.NoError(t, config.Storage.Flush())

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	transaction1 := modelpb.APMEvent{
		Trace: &trace1,
		Transaction: &modelpb.Transaction{
			Type:    "type",
			Id:      "0102030405060708",
			Sampled: true,
		},
	}
	span1 := modelpb.APMEvent{
		Trace: &trace1,
		Span: &modelpb.Span{
			Type: "type",
			Id:   "0102030405060709",
		},
	}
	transaction2 := modelpb.APMEvent{
		Trace: &trace2,
		Transaction: &modelpb.Transaction{
			Type:    "type",
			Id:      "0102030405060710",
			Sampled: true,
		},
	}
	span2 := modelpb.APMEvent{
		Trace: &trace2,
		Span: &modelpb.Span{
			Type: "type",
			Id:   "0102030405060711",
		},
	}

	batch := modelpb.Batch{&transaction1, &transaction2, &span1, &span2}
	err = processor.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	// Tail sampling decision already made. The first transaction and span should be
	// reported immediately, whereas the second ones should be written storage since
	// they were received after the trace sampling entry expired.
	assert.Equal(t, modelpb.Batch{&transaction1, &span1}, batch)

	monitoringtest.ExpectContainOtelMetrics(t, metricreader, map[string]any{
		"apm-server.sampling.tail.events.processed": 4,
		"apm-server.sampling.tail.events.stored":    2,
		"apm-server.sampling.tail.events.sampled":   2,
	})

	// Stop the processor and flush global storage so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	assert.NoError(t, config.Storage.Flush())
	reader := config.DB.NewBypassReadWriter()
	defer reader.Close()

	batch = nil
	err = reader.ReadTraceEvents(trace1.Id, &batch)
	assert.NoError(t, err)
	assert.Zero(t, batch)

	err = reader.ReadTraceEvents(trace2.Id, &batch)
	assert.NoError(t, err)
	assert.Empty(t, cmp.Diff(modelpb.Batch{&transaction2, &span2}, batch, protocmp.Transform()))
}

func TestProcessLocalTailSampling(t *testing.T) {
	for _, tc := range []struct {
		sampleRate float64
	}{
		{
			sampleRate: 0.5,
		},
		{
			// With 2 traces and 0.1 sample rate, ensure that we report 1 trace instead of 0 trace.
			sampleRate: 0.1,
		},
	} {
		t.Run(fmt.Sprintf("%f", tc.sampleRate), func(t *testing.T) {
			config, metricreader := newTempdirConfig(t)
			config.Policies = []sampling.Policy{{SampleRate: tc.sampleRate}}
			config.FlushInterval = 10 * time.Millisecond
			published := make(chan string)
			config.Elasticsearch = pubsubtest.Client(pubsubtest.PublisherChan(published), nil)

			processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)

			trace1 := modelpb.Trace{Id: "0102030405060708090a0b0c0d0e0f10"}
			trace2 := modelpb.Trace{Id: "0102030405060708090a0b0c0d0e0f11"}
			trace1Events := modelpb.Batch{{
				Trace: &trace1,
				Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
				Transaction: &modelpb.Transaction{
					Type:    "type",
					Id:      "0102030405060708",
					Sampled: true,
				},
			}, {
				Trace: &trace1,
				Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
				Span: &modelpb.Span{
					Type: "type",
					Id:   "0102030405060709",
				},
			}}
			trace2Events := modelpb.Batch{{
				Trace: &trace2,
				Event: &modelpb.Event{Duration: uint64(456 * time.Millisecond)},
				Transaction: &modelpb.Transaction{
					Type:    "type",
					Id:      "0102030405060710",
					Sampled: true,
				},
			}, {
				Trace: &trace2,
				Event: &modelpb.Event{Duration: uint64(456 * time.Millisecond)},
				Span: &modelpb.Span{
					Type: "type",
					Id:   "0102030405060711",
				},
			}}

			in := append(trace1Events[:], trace2Events...)
			err = processor.ProcessBatch(context.Background(), &in)
			require.NoError(t, err)
			assert.Empty(t, in)

			// Start periodic tail-sampling. We start the processor after processing
			// events to ensure all events are processed before any local sampling
			// decisions are made, such that we have a single tail-sampling decision
			// to check.
			go processor.Run()
			defer processor.Stop(context.Background())

			// We have configured 50% tail-sampling, so we expect a single trace ID
			// to be published. Sampling is non-deterministic (weighted random), so
			// we can't anticipate a specific trace ID.

			var sampledTraceID string
			select {
			case sampledTraceID = <-published:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for publication")
			}
			select {
			case <-published:
				t.Fatal("unexpected publication")
			case <-time.After(50 * time.Millisecond):
			}

			unsampledTraceID := trace2.Id
			sampledTraceEvents := trace1Events
			unsampledTraceEvents := trace2Events
			if sampledTraceID == trace2.Id {
				unsampledTraceID = trace1.Id
				unsampledTraceEvents = trace1Events
				sampledTraceEvents = trace2Events
			}

			monitoringtest.ExpectContainOtelMetrics(t, metricreader, map[string]any{
				"apm-server.sampling.tail.events.processed": 4,
				"apm-server.sampling.tail.events.stored":    4,
				"apm-server.sampling.tail.events.sampled":   2,
			})

			// Stop the processor and flush global storage so we can access the database.
			assert.NoError(t, processor.Stop(context.Background()))
			assert.NoError(t, config.Storage.Flush())
			reader := config.DB.NewBypassReadWriter()
			defer reader.Close()

			sampled, err := reader.IsTraceSampled(sampledTraceID)
			assert.NoError(t, err)
			assert.True(t, sampled)

			sampled, err = reader.IsTraceSampled(unsampledTraceID)
			assert.Equal(t, eventstorage.ErrNotFound, err)
			assert.False(t, sampled)

			var batch modelpb.Batch
			err = reader.ReadTraceEvents(sampledTraceID, &batch)
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(sampledTraceEvents, batch, protocmp.Transform()))

			// Even though the trace is unsampled, the events will be
			// available in storage until the TTL expires, as they're
			// written there first.
			batch = batch[:0]
			err = reader.ReadTraceEvents(unsampledTraceID, &batch)
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(unsampledTraceEvents, batch, protocmp.Transform()))
		})
	}

}

func TestProcessLocalTailSamplingUnsampled(t *testing.T) {
	config, metricreader := newTempdirConfig(t)
	config.FlushInterval = time.Minute
	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	// Process root transactions until one is rejected.
	traceIDs := make([]string, 10000)
	for i := range traceIDs {
		traceID := uuid.Must(uuid.NewV4()).String()
		traceIDs[i] = traceID
		batch := modelpb.Batch{{
			Trace: &modelpb.Trace{Id: traceID},
			Event: &modelpb.Event{Duration: uint64(time.Millisecond)},
			Transaction: &modelpb.Transaction{
				Type:    "type",
				Id:      traceID,
				Sampled: true,
			},
		}}
		err := processor.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
		assert.Empty(t, batch)

		// break out of the loop as soon as the first one is dropped.
		droppedEvents := getSum(t, metricreader, "apm-server.sampling.events.dropped")
		if droppedEvents != 0 {
			break
		}
	}

	// Stop the processor so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	assert.NoError(t, config.Storage.Flush())
	reader := config.DB.NewBypassReadWriter()
	defer reader.Close()

	var anyUnsampled bool
	for _, traceID := range traceIDs {
		sampled, err := reader.IsTraceSampled(traceID)
		if err == eventstorage.ErrNotFound {
			// No sampling decision made yet.
		} else {
			assert.NoError(t, err)
			if !sampled {
				anyUnsampled = true
				break
			}
		}
	}
	assert.True(t, anyUnsampled)
}

func TestProcessLocalTailSamplingPolicyOrder(t *testing.T) {
	config, _ := newTempdirConfig(t)
	config.Policies = []sampling.Policy{{
		PolicyCriteria: sampling.PolicyCriteria{TraceName: "trace_name"},
		SampleRate:     0.5,
	}, {
		PolicyCriteria: sampling.PolicyCriteria{ServiceName: "service_name"},
		SampleRate:     0.1,
	}, {
		PolicyCriteria: sampling.PolicyCriteria{},
		SampleRate:     0,
	}}
	config.FlushInterval = 10 * time.Millisecond
	published := make(chan string)
	config.Elasticsearch = pubsubtest.Client(pubsubtest.PublisherChan(published), nil)

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	// Send transactions which would match either policy defined above.
	rng := rand.New(rand.NewSource(0))
	service := modelpb.Service{Name: "service_name"}
	numTransactions := 100
	events := make(modelpb.Batch, numTransactions)
	for i := range events {
		var traceIDBytes [16]byte
		_, err := rng.Read(traceIDBytes[:])
		require.NoError(t, err)
		events[i] = &modelpb.APMEvent{
			Service: &service,
			Trace:   &modelpb.Trace{Id: fmt.Sprintf("%x", traceIDBytes[:])},
			Event:   &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
			Transaction: &modelpb.Transaction{
				Type:    "type",
				Name:    "trace_name",
				Id:      fmt.Sprintf("%x", traceIDBytes[8:]),
				Sampled: true,
			},
		}
	}

	err = processor.ProcessBatch(context.Background(), &events)
	require.NoError(t, err)
	assert.Empty(t, events)

	// Start periodic tail-sampling. We start the processor after processing
	// events to ensure all events are processed before any local sampling
	// decisions are made, such that we have a single tail-sampling decision
	// to check.
	go processor.Run()
	defer processor.Stop(context.Background())

	// The first matching policy should win, and sample 50%.
	for i := 0; i < numTransactions/2; i++ {
		select {
		case <-published:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for publication")
		}
	}
	select {
	case <-published:
		t.Fatal("unexpected publication")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestProcessRemoteTailSampling(t *testing.T) {
	config, metricreader := newTempdirConfig(t)
	config.Policies = []sampling.Policy{{SampleRate: 0.5}}
	config.FlushInterval = 10 * time.Millisecond

	var published []string
	var publisher pubsubtest.PublisherFunc = func(ctx context.Context, traceID string) error {
		published = append(published, traceID)
		return nil
	}
	subscriberChan := make(chan string)
	subscriber := pubsubtest.SubscriberChan(subscriberChan)
	config.Elasticsearch = pubsubtest.Client(publisher, subscriber)

	reported := make(chan modelpb.Batch)
	config.BatchProcessor = modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case reported <- batch.Clone():
			return nil
		}
	})

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	traceID1 := "0102030405060708090a0b0c0d0e0f10"
	traceID2 := "0102030405060708090a0b0c0d0e0f11"
	trace1Events := modelpb.Batch{{
		Trace: &modelpb.Trace{Id: traceID1},
		Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
		Span: &modelpb.Span{
			Type: "type",
			Id:   "0102030405060709",
		},
	}}

	in := trace1Events[:]
	err = processor.ProcessBatch(context.Background(), &in)
	require.NoError(t, err)
	assert.Empty(t, in)

	// Simulate receiving remote sampling decisions multiple times,
	// to show that we don't report duplicate events.
	subscriberChan <- traceID2
	subscriberChan <- traceID1
	subscriberChan <- traceID2
	subscriberChan <- traceID1

	var events modelpb.Batch
	select {
	case events = <-reported:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for reporting")
	}
	select {
	case <-reported:
		t.Fatal("unexpected reporting")
	case <-time.After(50 * time.Millisecond):
	}

	// Stop the processor and flush global storage so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	assert.NoError(t, config.Storage.Flush())
	assert.Empty(t, published) // remote decisions don't get republished

	monitoringtest.ExpectOtelMetrics(t, metricreader, map[string]any{
		"apm-server.sampling.tail.events.processed": 1,
		"apm-server.sampling.tail.events.stored":    1,
		"apm-server.sampling.tail.events.sampled":   1,
	})

	assert.Empty(t, cmp.Diff(trace1Events, events, protocmp.Transform()))

	reader := config.DB.NewBypassReadWriter()
	defer reader.Close()

	sampled, err := reader.IsTraceSampled(traceID1)
	assert.NoError(t, err)
	assert.True(t, sampled)

	sampled, err = reader.IsTraceSampled(traceID2)
	assert.NoError(t, err)
	assert.True(t, sampled)

	var batch modelpb.Batch
	err = reader.ReadTraceEvents(traceID1, &batch)
	assert.NoError(t, err)
	assert.Zero(t, batch) // events are deleted from local storage

	batch = modelpb.Batch{}
	err = reader.ReadTraceEvents(traceID2, &batch)
	assert.NoError(t, err)
	assert.Empty(t, batch)
}

type errorRW struct {
	err error
}

func (m errorRW) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	return m.err
}

func (m errorRW) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts eventstorage.WriterOpts) error {
	return m.err
}

func (m errorRW) WriteTraceSampled(traceID string, sampled bool, opts eventstorage.WriterOpts) error {
	return m.err
}

func (m errorRW) IsTraceSampled(traceID string) (bool, error) {
	return false, eventstorage.ErrNotFound
}

func (m errorRW) DeleteTraceEvent(traceID, id string) error {
	return m.err
}

func (m errorRW) Flush() error {
	return m.err
}

func TestProcessDiscardOnWriteFailure(t *testing.T) {
	for _, discard := range []bool{true, false} {
		t.Run(fmt.Sprintf("discard=%v", discard), func(t *testing.T) {
			config, _ := newTempdirConfig(t)
			config.DiscardOnWriteFailure = discard
			config.Storage = errorRW{err: errors.New("boom")}
			processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)
			go processor.Run()
			defer processor.Stop(context.Background())

			in := modelpb.Batch{{
				Trace: &modelpb.Trace{
					Id: "0102030405060708090a0b0c0d0e0f10",
				},
				Span: &modelpb.Span{
					Type: "type",
					Id:   "0102030405060708",
				},
			}}
			out := in[:]
			err = processor.ProcessBatch(context.Background(), &out)
			require.NoError(t, err)

			if discard {
				// Discarding by default
				assert.Empty(t, out)
			} else {
				// Indexing by default
				assert.Equal(t, in, out)
			}
		})
	}
}

func TestGroupsMonitoring(t *testing.T) {
	config, reader := newTempdirConfig(t)
	config.MaxDynamicServices = 5
	config.FlushInterval = time.Minute
	config.Policies[0].SampleRate = 0.99

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	for i := 0; i < config.MaxDynamicServices+2; i++ {
		err := processor.ProcessBatch(context.Background(), &modelpb.Batch{{
			Service: &modelpb.Service{Name: fmt.Sprintf("service_%d", i)},
			Trace:   &modelpb.Trace{Id: uuid.Must(uuid.NewV4()).String()},
			Event:   &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
			Transaction: &modelpb.Transaction{
				Type:    "type",
				Id:      "0102030405060709",
				Sampled: i < config.MaxDynamicServices+1,
			},
		}})
		require.NoError(t, err)
	}

	monitoringtest.ExpectOtelMetrics(t, reader, map[string]any{
		"apm-server.sampling.tail.dynamic_service_groups": config.MaxDynamicServices,
		"apm-server.sampling.tail.events.processed":       config.MaxDynamicServices + 2,
		"apm-server.sampling.tail.events.stored":          config.MaxDynamicServices,
		"apm-server.sampling.tail.events.dropped":         1, // final event dropped, after service limit reached
		"apm-server.sampling.tail.events.head_unsampled":  1,
	})
}

func TestStorageGC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	config, _ := newTempdirConfig(t)
	config.TTL = 10 * time.Millisecond
	config.FlushInterval = 10 * time.Millisecond

	writeBatch := func(n int) {
		config.StorageGCInterval = time.Hour // effectively disable
		processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)
		go processor.Run()
		defer processor.Stop(context.Background())
		for i := 0; i < n; i++ {
			traceID := uuid.Must(uuid.NewV4()).String()
			// Create a larger event to fill up the vlog faster, especially when it is above ValueThreshold
			batch := modelpb.Batch{{
				Trace: &modelpb.Trace{Id: traceID},
				Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
				Span: &modelpb.Span{
					Type:    strings.Repeat("a", 1000),
					Subtype: strings.Repeat("b", 1000),
					Id:      traceID,
					Name:    strings.Repeat("c", 1000),
				},
			}}
			err := processor.ProcessBatch(context.Background(), &batch)
			require.NoError(t, err)
			assert.Empty(t, batch)
		}
	}

	// Process spans until value log files have been created.
	// Garbage collection is disabled at this time.
	for len(vlogFilenames(config.StorageDir)) < 3 {
		writeBatch(2000)
	}

	config.StorageGCInterval = 10 * time.Millisecond
	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	// Wait for the first value log file to be garbage collected.
	var vlogs []string
	assert.Eventually(t, func() bool {
		vlogs = vlogFilenames(config.StorageDir)
		return len(vlogs) == 0 || vlogs[0] != "000000.vlog"
	}, 10*time.Second, 100*time.Millisecond, vlogs)
}

func TestStorageGCConcurrency(t *testing.T) {
	// This test ensures that TBS processor does not return an error
	// even when run concurrently e.g. in hot reload
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	config, _ := newTempdirConfig(t)
	config.TTL = 10 * time.Millisecond
	config.FlushInterval = 10 * time.Millisecond
	config.StorageGCInterval = 10 * time.Millisecond

	g := errgroup.Group{}
	for i := 0; i < 2; i++ {
		processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)
		g.Go(processor.Run)
		go func() {
			time.Sleep(time.Second)
			assert.NoError(t, processor.Stop(context.Background()))
		}()
	}
	assert.NoError(t, g.Wait())
}

func TestStorageLimit(t *testing.T) {
	// This test ensures that when tail sampling is configured with a hard
	// storage limit, the limit is respected once the size is available.
	// To update the database size during our test without waiting a full
	// minute, we store some span events, close and re-open the database, so
	// the size is updated.
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	writeBatch := func(n int, c sampling.Config, assertBatch func(b modelpb.Batch)) *sampling.Processor {
		processor, err := sampling.NewProcessor(c, logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)
		go processor.Run()
		defer processor.Stop(context.Background())
		batch := make(modelpb.Batch, 0, n)
		for i := 0; i < n; i++ {
			traceID := uuid.Must(uuid.NewV4()).String()
			batch = append(batch, &modelpb.APMEvent{
				Trace: &modelpb.Trace{Id: traceID},
				Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
				Span: &modelpb.Span{
					Type: "type",
					Id:   traceID,
				},
			})
		}
		err = processor.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
		assertBatch(batch)
		return processor
	}

	config, metricreader := newTempdirConfig(t)
	// Write 5K span events and close the DB to persist to disk the storage
	// size and assert that none are reported immediately.
	writeBatch(5000, config, func(b modelpb.Batch) {
		assert.Empty(t, b, fmt.Sprintf("expected empty but size is %d", len(b)))
	})
	assert.NoError(t, config.Storage.Flush())
	assert.NoError(t, config.DB.Close())

	// Open a new instance of the badgerDB and check the size.
	var err error
	config.DB, err = eventstorage.NewStorageManager(config.StorageDir)
	require.NoError(t, err)
	t.Cleanup(func() { config.DB.Close() })
	config.Storage = config.DB.NewReadWriter()

	lsm, vlog := config.DB.Size()
	assert.GreaterOrEqual(t, lsm+vlog, int64(1024))

	config.StorageLimit = 1024 // Set the storage limit to 1024 bytes.
	// Create a massive 150K span batch (per CPU) to trigger the badger error
	// Transaction too big, causing the ProcessBatch to report the some traces
	// immediately.
	// Rather than setting a static threshold, use the runtime.NumCPU as a
	// multiplier since the sharded writers use that variable and the more CPUs
	// we have, the more sharded writes we'll have, resulting in a greater buffer.
	// To avoid huge test time on large systems do this incrementally
	for i := 1; i < runtime.NumCPU(); i++ {
		writeBatch(150_000*i, config, func(b modelpb.Batch) {
			assert.NotEmpty(t, b)
		})

		// Ensure that there are some failed writes.
		failedWrites := getSum(t, metricreader, "apm-server.sampling.tail.events.failed_writes")
		t.Log(failedWrites)

		if failedWrites >= 1 {
			return
		}
	}

	t.Fatal("badger error never thrown")
}

func TestProcessRemoteTailSamplingPersistence(t *testing.T) {
	config, _ := newTempdirConfig(t)
	config.Policies = []sampling.Policy{{SampleRate: 0.5}}
	config.FlushInterval = 10 * time.Millisecond

	subscriberChan := make(chan string)
	subscriber := pubsubtest.SubscriberChan(subscriberChan)
	config.Elasticsearch = pubsubtest.Client(nil, subscriber)

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	// Wait for subscriber_position.json to be written to the storage directory.
	subscriberPositionFile := filepath.Join(config.StorageDir, "subscriber_position.json")
	data, info := waitFileModified(t, subscriberPositionFile, time.Time{})
	assert.Equal(t, "{}", string(data))

	subscriberChan <- "0102030405060708090a0b0c0d0e0f10"
	data, _ = waitFileModified(t, subscriberPositionFile, info.ModTime())
	assert.Equal(t, `{"index_name":1}`, string(data))
}

func TestDropLoop(t *testing.T) {
	// This test ensures that if badger is stuck at storage limit for TTL,
	// DB is dropped and recreated.
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	makeBatch := func(n int) modelpb.Batch {
		batch := make(modelpb.Batch, 0, n)
		for i := 0; i < n; i++ {
			traceID := uuid.Must(uuid.NewV4()).String()
			batch = append(batch, &modelpb.APMEvent{
				Trace: &modelpb.Trace{Id: traceID},
				Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
				Span: &modelpb.Span{
					Type: "type",
					Id:   traceID,
				},
			})
		}
		return batch
	}

	writeBatch := func(t *testing.T, n int, c sampling.Config, assertBatch func(b modelpb.Batch)) *sampling.Processor {
		processor, err := sampling.NewProcessor(c, logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)
		go processor.Run()
		defer processor.Stop(context.Background())
		batch := makeBatch(n)
		err = processor.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
		assertBatch(batch)
		return processor
	}

	for _, tc := range []struct {
		name                string
		subscriberPosExists bool
	}{
		{
			name:                "subscriber_position_not_exist",
			subscriberPosExists: false,
		},
		{
			name:                "subscriber_position_exists",
			subscriberPosExists: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			config, _ := newTempdirConfig(t)
			config.StorageGCInterval = time.Hour // effectively disable GC

			config.FlushInterval = 10 * time.Millisecond
			subscriberChan := make(chan string)
			subscriber := pubsubtest.SubscriberChan(subscriberChan)
			config.Elasticsearch = pubsubtest.Client(nil, subscriber)
			subscriberPositionFile := filepath.Join(config.StorageDir, "subscriber_position.json")

			// Write 5K span events and close the DB to persist to disk the storage
			// size and assert that none are reported immediately.
			writeBatch(t, 5000, config, func(b modelpb.Batch) {
				assert.Empty(t, b, fmt.Sprintf("expected empty but size is %d", len(b)))

				subscriberChan <- "0102030405060708090a0b0c0d0e0f10"
				assert.Eventually(t, func() bool {
					data, err := config.DB.ReadSubscriberPosition()
					return err == nil && string(data) == `{"index_name":1}`
				}, time.Second, 100*time.Millisecond)
			})
			assert.NoError(t, config.Storage.Flush())
			assert.NoError(t, config.DB.Close())

			if !tc.subscriberPosExists {
				err := os.Remove(subscriberPositionFile)
				assert.NoError(t, err)
			}

			func() {
				// Open a new instance of the badgerDB and check the size.
				var err error
				config.DB, err = eventstorage.NewStorageManager(config.StorageDir)
				require.NoError(t, err)
				t.Cleanup(func() { config.DB.Close() })
				config.Storage = config.DB.NewReadWriter()

				lsm, vlog := config.DB.Size()
				assert.Greater(t, lsm+vlog, int64(1024*1024))

				config.Elasticsearch = pubsubtest.Client(nil, nil) // disable pubsub

				// lower limit to trigger storage limit error
				// make it 5% higher than the existing size, and dropAndRecreate should kick in
				// because there's a hardcoded 90% threshold for storage limit
				config.StorageLimit = uint64(float64(lsm+vlog) * 1.05)
				config.TTL = time.Second
				processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
				require.NoError(t, err)
				go processor.Run()
				defer processor.Stop(context.Background())

				// wait for up to 1 minute for dropAndRecreate to kick in
				// no SST files after dropping DB and before first write
				var filenames []string
				assert.Eventually(t, func() bool {
					filenames = sstFilenames(config.StorageDir)
					return len(filenames) == 0
				}, 90*time.Second, 200*time.Millisecond, filenames)

				data, err := config.DB.ReadSubscriberPosition()
				assert.NoError(t, err)
				if tc.subscriberPosExists {
					assert.Equal(t, `{"index_name":1}`, string(data))
				} else {
					assert.Equal(t, "{}", string(data))
				}

				// try to write to new DB
				batch := makeBatch(10)
				err = processor.ProcessBatch(context.Background(), &batch)
				require.NoError(t, err)
			}()
			assert.NoError(t, config.DB.Close())
			assert.Greater(t, len(sstFilenames(config.StorageDir)), 0)
		})
	}
}

func TestReadSubscriberPositionFile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setupFile func(path string) error
	}{
		{
			name: "file not exist",
			setupFile: func(_ string) error {
				return nil
			},
		},
		{
			name: "valid json",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(`{}`), 0644)
			},
		},
		{
			name: "invalid json",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(`not_json`), 0644)
			},
		},
		{
			name: "bad perm",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte{}, 0000)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tempdirConfig, _ := newTempdirConfig(t)

			err := tc.setupFile(filepath.Join(tempdirConfig.StorageDir, "subscriber_position.json"))
			require.NoError(t, err)

			processor, err := sampling.NewProcessor(tempdirConfig, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)

			ret := make(chan error)
			go func() {
				ret <- processor.Run()
			}()
			processor.Stop(context.Background())
			assert.NoError(t, <-ret)
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	config, _ := newTempdirConfig(t)
	sampleRate := 0.5
	config.Policies = []sampling.Policy{{SampleRate: sampleRate}}
	config.FlushInterval = time.Minute // disable finalize

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()

	totalTraces := 100
	traceIDGen := func(i int) string { return fmt.Sprintf("trace%d", i) }

	var batch modelpb.Batch
	for i := 0; i < totalTraces; i++ {
		batch = append(batch, &modelpb.APMEvent{
			Trace: &modelpb.Trace{Id: traceIDGen(i)},
			Transaction: &modelpb.Transaction{
				Type:    "type",
				Id:      fmt.Sprintf("tx%d", i),
				Sampled: true,
			},
		})
	}

	assert.NoError(t, processor.ProcessBatch(context.Background(), &batch))
	assert.Empty(t, batch)
	assert.NoError(t, processor.Stop(context.Background()))
	assert.NoError(t, config.Storage.Flush())

	reader := config.DB.NewBypassReadWriter()
	defer reader.Close()

	var count int
	for i := 0; i < totalTraces; i++ {
		if ok, _ := reader.IsTraceSampled(traceIDGen(i)); ok {
			count++
		}
	}
	assert.Equal(t, int(sampleRate*float64(totalTraces)), count)
}

func newTempdirConfig(tb testing.TB) (sampling.Config, sdkmetric.Reader) {
	tempdir, err := os.MkdirTemp("", "samplingtest")
	require.NoError(tb, err)
	tb.Cleanup(func() { os.RemoveAll(tempdir) })

	badgerDB, err := eventstorage.NewStorageManager(tempdir)
	require.NoError(tb, err)
	tb.Cleanup(func() { badgerDB.Close() })

	storage := badgerDB.NewReadWriter()

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	return sampling.Config{
		BatchProcessor: modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error { return nil }),
		MeterProvider:  mp,
		LocalSamplingConfig: sampling.LocalSamplingConfig{
			FlushInterval:         time.Second,
			MaxDynamicServices:    1000,
			IngestRateDecayFactor: 0.9,
			Policies: []sampling.Policy{
				{SampleRate: 0.1},
			},
		},
		RemoteSamplingConfig: sampling.RemoteSamplingConfig{
			Elasticsearch: pubsubtest.Client(nil, nil),
			SampledTracesDataStream: sampling.DataStreamConfig{
				Type:      "traces",
				Dataset:   "sampled",
				Namespace: "testing",
			},
			UUID: "local-apm-server",
		},
		StorageConfig: sampling.StorageConfig{
			DB:                badgerDB,
			Storage:           storage,
			StorageDir:        tempdir,
			StorageGCInterval: time.Second,
			TTL:               30 * time.Minute,
			StorageLimit:      0, // No storage limit.
		},
	}, reader
}

func getSum(t testing.TB, reader sdkmetric.Reader, name string) int64 {
	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m.Data.(metricdata.Sum[int64]).DataPoints[0].Value
			}
		}
	}

	return 0
}

// waitFileModified waits up to 10 seconds for filename to exist and for its
// modification time to be greater than "after", and returns the file content
// and file info (including modification time).
func waitFileModified(tb testing.TB, filename string, after time.Time) ([]byte, os.FileInfo) {
	// Wait for subscriber_position.json to be written to the storage directory.
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {

		select {
		case <-ticker.C:
			info, err := os.Stat(filename)
			if errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				tb.Fatal(err)
			}
			if info.ModTime().After(after) {
				data, err := os.ReadFile(filename)
				if err != nil {
					tb.Fatal(err)
				}
				return data, info
			}
		case <-timeout.C:
			tb.Fatalf("timed out waiting for %q to be modified", filename)
		}
	}
}

func vlogFilenames(storageDir string) []string {
	entries, _ := os.ReadDir(storageDir)

	var vlogs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".vlog") {
			vlogs = append(vlogs, name)
		}
	}
	sort.Strings(vlogs)
	return vlogs
}

func sstFilenames(storageDir string) []string {
	entries, _ := os.ReadDir(storageDir)

	var ssts []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".sst") {
			ssts = append(ssts, name)
		}
	}
	sort.Strings(ssts)
	return ssts
}
