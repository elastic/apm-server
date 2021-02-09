// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package sampling_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub/pubsubtest"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestProcessUnsampled(t *testing.T) {
	processor, err := sampling.NewProcessor(newTempdirConfig(t))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	transaction := &model.Transaction{
		TraceID: "0102030405060708090a0b0c0d0e0f10",
		ID:      "0102030405060708",
		Sampled: newBool(false),
	}
	in := []transform.Transformable{transaction}
	out, err := processor.ProcessTransformables(context.Background(), in)
	require.NoError(t, err)

	// Unsampled transaction should be reported immediately.
	assert.Equal(t, in, out)
}

func TestProcessAlreadyTailSampled(t *testing.T) {
	config := newTempdirConfig(t)

	// Seed event storage with a tail-sampling decisions, to show that
	// subsequent events in the trace will be reported immediately.
	traceID1 := "0102030405060708090a0b0c0d0e0f10"
	traceID2 := "0102030405060708090a0b0c0d0e0f11"
	withBadger(t, config.StorageDir, func(db *badger.DB) {
		storage := eventstorage.New(db, eventstorage.JSONCodec{}, time.Minute)
		writer := storage.NewReadWriter()
		defer writer.Close()
		assert.NoError(t, writer.WriteTraceSampled(traceID1, true))
		assert.NoError(t, writer.Flush())

		storage = eventstorage.New(db, eventstorage.JSONCodec{}, -1) // expire immediately
		writer = storage.NewReadWriter()
		defer writer.Close()
		assert.NoError(t, writer.WriteTraceSampled(traceID2, true))
		assert.NoError(t, writer.Flush())
	})

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	transaction1 := &model.Transaction{
		TraceID: traceID1,
		ID:      "0102030405060708",
	}
	span1 := &model.Span{
		TraceID: traceID1,
		ID:      "0102030405060709",
	}
	transaction2 := &model.Transaction{
		TraceID: traceID2,
		ID:      "0102030405060710",
	}
	span2 := &model.Span{
		TraceID: traceID2,
		ID:      "0102030405060711",
	}

	in := []transform.Transformable{transaction1, transaction2, span1, span2}
	out, err := processor.ProcessTransformables(context.Background(), in)
	require.NoError(t, err)

	// Tail sampling decision already made. The first transaction and span should be
	// reported immediately, whereas the second ones should be written storage since
	// they were received after the trace sampling entry expired.
	assert.Equal(t, []transform.Transformable{transaction1, span1}, out)

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["sampling.events.processed"] = 4
	expectedMonitoring.Ints["sampling.events.stored"] = 2
	expectedMonitoring.Ints["sampling.events.dropped"] = 0
	assertMonitoring(t, processor, expectedMonitoring, `sampling.events.*`)

	// Stop the processor so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	withBadger(t, config.StorageDir, func(db *badger.DB) {
		storage := eventstorage.New(db, eventstorage.JSONCodec{}, time.Minute)
		reader := storage.NewReadWriter()
		defer reader.Close()

		var batch model.Batch
		err := reader.ReadEvents(traceID1, &batch)
		assert.NoError(t, err)
		assert.Zero(t, batch)

		err = reader.ReadEvents(traceID2, &batch)
		assert.NoError(t, err)
		assert.Equal(t, model.Batch{
			Spans:        []*model.Span{span2},
			Transactions: []*model.Transaction{transaction2},
		}, batch)
	})
}

func TestProcessLocalTailSampling(t *testing.T) {
	config := newTempdirConfig(t)
	config.Policies = []sampling.Policy{{SampleRate: 0.5}}
	config.FlushInterval = 10 * time.Millisecond
	published := make(chan string)
	config.Elasticsearch = pubsubtest.Client(pubsubtest.PublisherChan(published), nil)

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)

	traceID1 := "0102030405060708090a0b0c0d0e0f10"
	traceID2 := "0102030405060708090a0b0c0d0e0f11"
	trace1Events := model.Batch{
		Transactions: []*model.Transaction{{
			TraceID:  traceID1,
			ID:       "0102030405060708",
			Duration: 123,
		}},
		Spans: []*model.Span{{
			TraceID:  traceID1,
			ID:       "0102030405060709",
			Duration: 123,
		}},
	}
	trace2Events := model.Batch{
		Transactions: []*model.Transaction{{
			TraceID:  traceID2,
			ID:       "0102030405060710",
			Duration: 456,
		}},
		Spans: []*model.Span{{
			TraceID:  traceID2,
			ID:       "0102030405060711",
			Duration: 456,
		}},
	}

	in := append(trace1Events.Transformables(), trace2Events.Transformables()...)
	out, err := processor.ProcessTransformables(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, out)

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

	unsampledTraceID := traceID2
	sampledTraceEvents := trace1Events
	unsampledTraceEvents := trace2Events
	if sampledTraceID == traceID2 {
		unsampledTraceID = traceID1
		unsampledTraceEvents = trace1Events
		sampledTraceEvents = trace2Events
	}

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["sampling.events.processed"] = 4
	expectedMonitoring.Ints["sampling.events.stored"] = 4
	expectedMonitoring.Ints["sampling.events.dropped"] = 0
	assertMonitoring(t, processor, expectedMonitoring, `sampling.events.*`)

	// Stop the processor so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	withBadger(t, config.StorageDir, func(db *badger.DB) {
		storage := eventstorage.New(db, eventstorage.JSONCodec{}, time.Minute)
		reader := storage.NewReadWriter()
		defer reader.Close()

		sampled, err := reader.IsTraceSampled(sampledTraceID)
		assert.NoError(t, err)
		assert.True(t, sampled)

		sampled, err = reader.IsTraceSampled(unsampledTraceID)
		assert.Equal(t, eventstorage.ErrNotFound, err)
		assert.False(t, sampled)

		var batch model.Batch
		err = reader.ReadEvents(sampledTraceID, &batch)
		assert.NoError(t, err)
		assert.Equal(t, sampledTraceEvents, batch)

		// Even though the trace is unsampled, the events will be
		// available in storage until the TTL expires, as they're
		// written there first.
		batch.Reset()
		err = reader.ReadEvents(unsampledTraceID, &batch)
		assert.NoError(t, err)
		assert.Equal(t, unsampledTraceEvents, batch)
	})
}

func TestProcessLocalTailSamplingUnsampled(t *testing.T) {
	config := newTempdirConfig(t)
	config.FlushInterval = time.Minute
	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	// Process root transactions until one is rejected.
	traceIDs := make([]string, 10000)
	for i := range traceIDs {
		traceID := uuid.Must(uuid.NewV4()).String()
		traceIDs[i] = traceID
		tx := &model.Transaction{
			TraceID:  traceID,
			ID:       traceID,
			Duration: 1,
		}
		out, err := processor.ProcessTransformables(context.Background(), []transform.Transformable{tx})
		require.NoError(t, err)
		assert.Empty(t, out)
	}

	// Stop the processor so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	withBadger(t, config.StorageDir, func(db *badger.DB) {
		storage := eventstorage.New(db, eventstorage.JSONCodec{}, time.Minute)
		reader := storage.NewReadWriter()
		defer reader.Close()

		var anyUnsampled bool
		for _, traceID := range traceIDs {
			sampled, err := reader.IsTraceSampled(traceID)
			if err == eventstorage.ErrNotFound {
				// No sampling decision made yet.
			} else {
				assert.NoError(t, err)
				assert.False(t, sampled)
				anyUnsampled = true
				break
			}
		}
		assert.True(t, anyUnsampled)
	})
}

func TestProcessLocalTailSamplingPolicyOrder(t *testing.T) {
	config := newTempdirConfig(t)
	config.Policies = []sampling.Policy{{
		PolicyCriteria: sampling.PolicyCriteria{TraceName: "trace_name"},
		SampleRate:     0.5,
	}, {
		PolicyCriteria: sampling.PolicyCriteria{ServiceName: "service_name"},
		SampleRate:     0.1,
	}}
	config.FlushInterval = 10 * time.Millisecond
	published := make(chan string)
	config.Elasticsearch = pubsubtest.Client(pubsubtest.PublisherChan(published), nil)

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)

	// Send transactions which would match either policy defined above.
	rng := rand.New(rand.NewSource(0))
	metadata := model.Metadata{Service: model.Service{Name: "service_name"}}
	var events []transform.Transformable
	for i := 0; i < 100; i++ {
		var traceIDBytes [16]byte
		_, err := rng.Read(traceIDBytes[:])
		require.NoError(t, err)
		events = append(events, &model.Transaction{
			Metadata: metadata,
			Name:     "trace_name",
			TraceID:  fmt.Sprintf("%x", traceIDBytes[:]),
			ID:       fmt.Sprintf("%x", traceIDBytes[8:]),
			Duration: 123,
		})
	}

	out, err := processor.ProcessTransformables(context.Background(), events)
	require.NoError(t, err)
	assert.Empty(t, out)

	// Start periodic tail-sampling. We start the processor after processing
	// events to ensure all events are processed before any local sampling
	// decisions are made, such that we have a single tail-sampling decision
	// to check.
	go processor.Run()
	defer processor.Stop(context.Background())

	// The first matching policy should win, and sample 50%.
	for i := 0; i < len(events)/2; i++ {
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
	config := newTempdirConfig(t)
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

	reported := make(chan []transform.Transformable)
	config.Reporter = func(ctx context.Context, req publish.PendingReq) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case reported <- req.Transformables:
			return nil
		}
	}

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	traceID1 := "0102030405060708090a0b0c0d0e0f10"
	traceID2 := "0102030405060708090a0b0c0d0e0f11"
	trace1Events := model.Batch{
		Spans: []*model.Span{{
			TraceID:  traceID1,
			ID:       "0102030405060709",
			Duration: 123,
		}},
	}

	in := trace1Events.Transformables()
	out, err := processor.ProcessTransformables(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, out)

	// Simulate receiving remote sampling decisions multiple times,
	// to show that we don't report duplicate events.
	subscriberChan <- traceID2
	subscriberChan <- traceID1
	subscriberChan <- traceID2
	subscriberChan <- traceID1

	var events []transform.Transformable
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

	// Stop the processor so we can access the database.
	assert.NoError(t, processor.Stop(context.Background()))
	assert.Empty(t, published) // remote decisions don't get republished

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["sampling.events.processed"] = 1
	expectedMonitoring.Ints["sampling.events.stored"] = 1
	expectedMonitoring.Ints["sampling.events.dropped"] = 0
	assertMonitoring(t, processor, expectedMonitoring, `sampling.events.*`)

	assert.Equal(t, trace1Events.Transformables(), events)

	withBadger(t, config.StorageDir, func(db *badger.DB) {
		storage := eventstorage.New(db, eventstorage.JSONCodec{}, time.Minute)
		reader := storage.NewReadWriter()
		defer reader.Close()

		sampled, err := reader.IsTraceSampled(traceID1)
		assert.NoError(t, err)
		assert.True(t, sampled)

		sampled, err = reader.IsTraceSampled(traceID2)
		assert.NoError(t, err)
		assert.True(t, sampled)

		var batch model.Batch
		err = reader.ReadEvents(traceID1, &batch)
		assert.NoError(t, err)
		assert.Zero(t, batch) // events are deleted from local storage

		batch = model.Batch{}
		err = reader.ReadEvents(traceID2, &batch)
		assert.NoError(t, err)
		assert.Zero(t, batch)
	})
}

func TestGroupsMonitoring(t *testing.T) {
	config := newTempdirConfig(t)
	config.MaxDynamicServices = 5
	config.FlushInterval = time.Minute
	config.Policies[0].SampleRate = 0.99

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	for i := 0; i < config.MaxDynamicServices+1; i++ {
		_, err := processor.ProcessTransformables(context.Background(), []transform.Transformable{&model.Transaction{
			Metadata: model.Metadata{
				Service: model.Service{Name: fmt.Sprintf("service_%d", i)},
			},
			TraceID:  uuid.Must(uuid.NewV4()).String(),
			ID:       "0102030405060709",
			Duration: 123,
		}})
		require.NoError(t, err)
	}

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["sampling.dynamic_service_groups"] = int64(config.MaxDynamicServices)
	expectedMonitoring.Ints["sampling.events.processed"] = int64(config.MaxDynamicServices) + 1
	expectedMonitoring.Ints["sampling.events.stored"] = int64(config.MaxDynamicServices)
	expectedMonitoring.Ints["sampling.events.dropped"] = 1 // final event dropped, after service limit reached
	assertMonitoring(t, processor, expectedMonitoring, `sampling.events.*`, `sampling.dynamic_service_groups`)
}

func TestStorageMonitoring(t *testing.T) {
	config := newTempdirConfig(t)

	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())
	for i := 0; i < 100; i++ {
		traceID := uuid.Must(uuid.NewV4()).String()
		out, err := processor.ProcessTransformables(context.Background(), []transform.Transformable{&model.Transaction{
			TraceID:  traceID,
			ID:       traceID,
			Duration: 123,
		}})
		require.NoError(t, err)
		assert.Empty(t, out)
	}

	// Stop the processor and create a new one, which will reopen storage
	// and calculate the storage size. Otherwise we must wait for a minute
	// (hard-coded in badger) for storage metrics to be updated.
	processor.Stop(context.Background())
	processor, err = sampling.NewProcessor(config)
	require.NoError(t, err)

	metrics := collectProcessorMetrics(processor)
	assert.NotZero(t, metrics.Ints, "sampling.storage.lsm_size")
	assert.NotZero(t, metrics.Ints, "sampling.storage.value_log_size")
}

func TestStorageGC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow test")
	}

	config := newTempdirConfig(t)
	config.TTL = 10 * time.Millisecond
	config.FlushInterval = 10 * time.Millisecond
	config.ValueLogFileSize = 1024 * 1024

	writeBatch := func(n int) {
		config.StorageGCInterval = time.Minute // effectively disable
		processor, err := sampling.NewProcessor(config)
		require.NoError(t, err)
		go processor.Run()
		defer processor.Stop(context.Background())
		for i := 0; i < n; i++ {
			traceID := uuid.Must(uuid.NewV4()).String()
			out, err := processor.ProcessTransformables(context.Background(), []transform.Transformable{&model.Span{
				TraceID:  traceID,
				ID:       traceID,
				Duration: 123,
			}})
			require.NoError(t, err)
			assert.Empty(t, out)
		}
	}

	vlogFilenames := func() []string {
		dir, _ := os.Open(config.StorageDir)
		names, _ := dir.Readdirnames(-1)
		defer dir.Close()

		var vlogs []string
		for _, name := range names {
			if strings.HasSuffix(name, ".vlog") {
				vlogs = append(vlogs, name)
			}
		}
		sort.Strings(vlogs)
		return vlogs
	}

	// Process spans until more than one value log file has been created,
	// but the first one does not exist (has been garbage collected).
	for len(vlogFilenames()) < 2 {
		writeBatch(50000)
	}

	config.StorageGCInterval = 10 * time.Millisecond
	processor, err := sampling.NewProcessor(config)
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		vlogs := vlogFilenames()
		if len(vlogs) == 0 || vlogs[0] != "000000.vlog" {
			// garbage collected
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for value log garbage collection")
}

func withBadger(tb testing.TB, storageDir string, f func(db *badger.DB)) {
	badgerOpts := badger.DefaultOptions(storageDir)
	badgerOpts.Logger = nil
	db, err := badger.Open(badgerOpts)
	require.NoError(tb, err)
	f(db)
	assert.NoError(tb, db.Close())
}

func newTempdirConfig(tb testing.TB) sampling.Config {
	tempdir, err := ioutil.TempDir("", "samplingtest")
	require.NoError(tb, err)
	tb.Cleanup(func() { os.RemoveAll(tempdir) })
	return sampling.Config{
		BeatID:   "local-apm-server",
		Reporter: func(ctx context.Context, req publish.PendingReq) error { return nil },
		LocalSamplingConfig: sampling.LocalSamplingConfig{
			FlushInterval:         time.Second,
			MaxDynamicServices:    1000,
			IngestRateDecayFactor: 0.9,
			Policies: []sampling.Policy{
				{SampleRate: 0.1},
			},
		},
		RemoteSamplingConfig: sampling.RemoteSamplingConfig{
			Elasticsearch:      pubsubtest.Client(nil, nil),
			SampledTracesIndex: "apm-sampled-traces",
		},
		StorageConfig: sampling.StorageConfig{
			StorageDir:        tempdir,
			StorageGCInterval: time.Second,
			TTL:               30 * time.Minute,
		},
	}
}

func assertMonitoring(t testing.TB, p *sampling.Processor, expected monitoring.FlatSnapshot, matches ...string) {
	t.Helper()
	actual := collectProcessorMetrics(p)
	matchAny := func(k string) bool { return true }
	if len(matches) > 0 {
		matchAny = func(k string) bool {
			for _, pattern := range matches {
				matched, err := path.Match(pattern, k)
				if err != nil {
					panic(err)
				}
				if matched {
					return true
				}
			}
			return false
		}
	}
	for k := range actual.Bools {
		if !matchAny(k) {
			delete(actual.Bools, k)
		}
	}
	for k := range actual.Ints {
		if !matchAny(k) {
			delete(actual.Ints, k)
		}
	}
	for k := range actual.Floats {
		if !matchAny(k) {
			delete(actual.Floats, k)
		}
	}
	for k := range actual.Strings {
		if !matchAny(k) {
			delete(actual.Strings, k)
		}
	}
	for k := range actual.StringSlices {
		if !matchAny(k) {
			delete(actual.StringSlices, k)
		}
	}
	assert.Equal(t, expected, actual)
}

func collectProcessorMetrics(p *sampling.Processor) monitoring.FlatSnapshot {
	registry := monitoring.NewRegistry()
	monitoring.NewFunc(registry, "sampling", p.CollectMonitoring)
	return monitoring.CollectFlatSnapshot(
		registry,
		monitoring.Full,
		false, // expvar
	)
}

func newBool(v bool) *bool {
	return &v
}
