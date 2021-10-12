// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package txmetrics_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

func TestNewAggregatorConfigInvalid(t *testing.T) {
	batchProcessor := makeErrBatchProcessor(nil)

	type test struct {
		config txmetrics.AggregatorConfig
		err    string
	}

	for _, test := range []test{{
		config: txmetrics.AggregatorConfig{},
		err:    "BatchProcessor unspecified",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor: batchProcessor,
		},
		err: "MaxTransactionGroups unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor:       batchProcessor,
			MaxTransactionGroups: 1,
		},
		err: "MetricsInterval unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor:                 batchProcessor,
			MaxTransactionGroups:           1,
			MetricsInterval:                time.Nanosecond,
			HDRHistogramSignificantFigures: 6,
		},
		err: "HDRHistogramSignificantFigures (6) outside range [1,5]",
	}} {
		agg, err := txmetrics.NewAggregator(test.config)
		require.Error(t, err)
		require.Nil(t, agg)
		assert.EqualError(t, err, "invalid aggregator config: "+test.err)
	}
}

func TestProcessTransformablesOverflow(t *testing.T) {
	batches := make(chan model.Batch, 1)

	core, observed := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("foo", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MetricsInterval:                time.Microsecond,
		HDRHistogramSignificantFigures: 1,
		Logger:                         logger,
	})
	require.NoError(t, err)

	// The first two transaction groups will not require immediate publication,
	// as we have configured the txmetrics with a maximum of two buckets.
	batch := make(model.Batch, 20)
	for i := 0; i < len(batch); i += 2 {
		batch[i] = model.APMEvent{
			Processor:   model.TransactionProcessor,
			Transaction: &model.Transaction{Name: "foo", RepresentativeCount: 1},
		}
		batch[i+1] = model.APMEvent{
			Processor:   model.TransactionProcessor,
			Transaction: &model.Transaction{Name: "bar", RepresentativeCount: 1},
		}
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))

	// The third transaction group will return a metricset for immediate publication.
	for i := 0; i < 2; i++ {
		batch = append(batch, model.APMEvent{
			Processor: model.TransactionProcessor,
			Event:     model.Event{Duration: time.Minute},
			Transaction: &model.Transaction{
				Name:                "baz",
				RepresentativeCount: 1,
			},
		})
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	metricsets := batchMetricsets(t, batch)
	assert.Len(t, metricsets, 2)

	for _, m := range metricsets {
		assert.Equal(t, model.APMEvent{
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:                 "transaction",
				TimeseriesInstanceID: ":baz:9bf2d21a00716e4a",
				DocCount:             1,
			},
			Transaction: &model.Transaction{
				Name: "baz",
				Root: true,
				DurationHistogram: model.Histogram{
					Counts: []int64{1},
					Values: []float64{float64(time.Minute / time.Microsecond)},
				},
			},
		}, m)
	}

	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["txmetrics.active_groups"] = 2
	expectedMonitoring.Ints["txmetrics.overflowed"] = 2 // third group is processed twice

	registry := monitoring.NewRegistry()
	monitoring.NewFunc(registry, "txmetrics", agg.CollectMonitoring)
	assert.Equal(t, expectedMonitoring, monitoring.CollectFlatSnapshot(
		registry,
		monitoring.Full,
		false, // expvar
	))

	overflowLogEntries := observed.FilterMessageSnippet("Transaction group limit reached")
	assert.Equal(t, 1, overflowLogEntries.Len()) // rate limited
}

func TestAggregatorRun(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MetricsInterval:                10 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
		metricset := agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "T-1000",
				RepresentativeCount: 1,
			},
		})
		require.Zero(t, metricset)
	}
	for i := 0; i < 800; i++ {
		metricset := agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "T-800",
				RepresentativeCount: 1,
			},
		})
		require.Zero(t, metricset)
	}

	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	require.Len(t, metricsets, 2)
	sort.Slice(metricsets, func(i, j int) bool {
		return metricsets[i].Transaction.Name < metricsets[j].Transaction.Name
	})

	assert.Equal(t, "T-1000", metricsets[0].Transaction.Name)
	assert.Equal(t, []int64{1000}, metricsets[0].Transaction.DurationHistogram.Counts)
	assert.Equal(t, "T-800", metricsets[1].Transaction.Name)
	assert.Equal(t, []int64{800}, metricsets[1].Transaction.DurationHistogram.Counts)

	select {
	case <-batches:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregatorRunPublishErrors(t *testing.T) {
	batches := make(chan model.Batch, 1)
	chanBatchProcessor := makeChanBatchProcessor(batches)
	processBatchErr := errors.New("report failed")
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		if err := chanBatchProcessor(ctx, batch); err != nil {
			return err
		}
		return processBatchErr
	}

	core, observed := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("foo", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 batchProcessor,
		MaxTransactionGroups:           2,
		MetricsInterval:                10 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
		Logger:                         logger,
	})
	require.NoError(t, err)

	go agg.Run()
	defer agg.Stop(context.Background())

	for i := 0; i < 2; i++ {
		metricset := agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "T-1000",
				RepresentativeCount: 1,
			},
		})
		require.Zero(t, metricset)
		expectBatch(t, batches)
	}

	// Wait for aggregator to stop before checking logs, to ensure we don't race with logging.
	assert.NoError(t, agg.Stop(context.Background()))

	logs := observed.FilterMessageSnippet("report failed").All()
	assert.Len(t, logs, 2)
	for _, record := range logs {
		require.Len(t, record.Context, 1)
		assert.Equal(t, "error", record.Context[0].Key)
		assert.Equal(t, processBatchErr, record.Context[0].Interface)
	}
}

func TestAggregateRepresentativeCount(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           1,
		MetricsInterval:                time.Microsecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	// Record a transaction group so subsequent calls yield immediate metricsets,
	// and to demonstrate that fractional transaction counts are accumulated.
	agg.AggregateTransaction(model.APMEvent{
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{Name: "fnord", RepresentativeCount: 1},
	})
	agg.AggregateTransaction(model.APMEvent{
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{Name: "fnord", RepresentativeCount: 1.5},
	})

	// For non-positive RepresentativeCounts, no metrics will be accumulated.
	for _, representativeCount := range []float64{-1, 0} {
		m := agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "foo",
				RepresentativeCount: representativeCount,
			},
		})
		assert.Zero(t, m)
	}

	for _, test := range []struct {
		representativeCount float64
		expectedCount       int64
	}{{
		representativeCount: 1,
		expectedCount:       1,
	}, {
		representativeCount: 2,
		expectedCount:       2,
	}, {
		representativeCount: 1.50, // round half away from zero
		expectedCount:       2,
	}} {
		m := agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "foo",
				RepresentativeCount: test.representativeCount,
			},
		})
		require.NotNil(t, m.Metricset)

		m.Timestamp = time.Time{}
		assert.Equal(t, model.APMEvent{
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:                 "transaction",
				TimeseriesInstanceID: ":foo:9f7a6aa5754581fe",
				DocCount:             test.expectedCount,
			},
			Transaction: &model.Transaction{
				Name: "foo",
				Root: true,
				DurationHistogram: model.Histogram{
					Counts: []int64{test.expectedCount},
					Values: []float64{0},
				},
			},
		}, m)
	}

	go agg.Run()
	defer agg.Stop(context.Background())

	// Check the fractional transaction counts for the "fnord" transaction
	// group were accumulated with some degree of accuracy. i.e. we should
	// receive round(1+1.5)=3; the fractional values should not have been
	// truncated.
	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	require.Len(t, metricsets, 1)
	require.Nil(t, metricsets[0].Metricset.Samples)
	require.NotNil(t, metricsets[0].Transaction)
	durationHistogram := metricsets[0].Transaction.DurationHistogram
	assert.Equal(t, []int64{3 /*round(1+1.5)*/}, durationHistogram.Counts)
}

func TestAggregateTimestamp(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MetricsInterval:                30 * time.Second,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0)
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		agg.AggregateTransaction(model.APMEvent{
			Timestamp:   ts,
			Processor:   model.TransactionProcessor,
			Transaction: &model.Transaction{Name: "name", RepresentativeCount: 1},
		})
	}

	go agg.Run()
	err = agg.Stop(context.Background()) // stop to flush
	require.NoError(t, err)

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	require.Len(t, metricsets, 2)
	sort.Slice(metricsets, func(i, j int) bool {
		return metricsets[i].Timestamp.Before(metricsets[j].Timestamp)
	})
	assert.Equal(t, t0, metricsets[0].Timestamp)
	assert.Equal(t, t0.Add(30*time.Second), metricsets[1].Timestamp)
}

func TestHDRHistogramSignificantFigures(t *testing.T) {
	testHDRHistogramSignificantFigures(t, 1)
	testHDRHistogramSignificantFigures(t, 2)
	testHDRHistogramSignificantFigures(t, 3)
	testHDRHistogramSignificantFigures(t, 4)
	testHDRHistogramSignificantFigures(t, 5)
}

func testHDRHistogramSignificantFigures(t *testing.T, sigfigs int) {
	t.Run(fmt.Sprintf("%d_sigfigs", sigfigs), func(t *testing.T) {
		batches := make(chan model.Batch, 1)
		agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
			BatchProcessor:                 makeChanBatchProcessor(batches),
			MaxTransactionGroups:           2,
			MetricsInterval:                10 * time.Millisecond,
			HDRHistogramSignificantFigures: sigfigs,
		})
		require.NoError(t, err)

		// The following values will be recorded in either 1, 2, 3, 4, or 5
		// buckets according to the configured number of significant figures.
		for _, duration := range []time.Duration{
			100000 * time.Microsecond,
			101000 * time.Microsecond,
			101100 * time.Microsecond,
			101110 * time.Microsecond,
			101111 * time.Microsecond,
		} {
			metricset := agg.AggregateTransaction(model.APMEvent{
				Processor: model.TransactionProcessor,
				Event:     model.Event{Duration: duration},
				Transaction: &model.Transaction{
					Name:                "T-1000",
					RepresentativeCount: 1,
				},
			})
			require.Zero(t, metricset)
		}

		go agg.Run()
		defer agg.Stop(context.Background())

		batch := expectBatch(t, batches)
		metricsets := batchMetricsets(t, batch)
		require.Len(t, metricsets, 1)

		require.Nil(t, metricsets[0].Metricset.Samples)
		require.NotNil(t, metricsets[0].Transaction)
		durationHistogram := metricsets[0].Transaction.DurationHistogram
		assert.Len(t, durationHistogram.Counts, len(durationHistogram.Values))
		assert.Len(t, durationHistogram.Counts, sigfigs)
	})
}

func TestAggregationFields(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           1000,
		MetricsInterval:                100 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)
	go agg.Run()
	defer agg.Stop(context.Background())

	input := model.APMEvent{
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{RepresentativeCount: 1},
	}
	inputFields := []*string{
		&input.Transaction.Name,
		&input.Transaction.Result,
		&input.Transaction.Type,
		&input.Event.Outcome,
		&input.Agent.Name,
		&input.Service.Environment,
		&input.Service.Name,
		&input.Service.Version,
		&input.Service.Node.Name,
		&input.Container.ID,
		&input.Kubernetes.PodName,
		&input.Cloud.Provider,
		&input.Cloud.Region,
		&input.Cloud.AvailabilityZone,
	}

	var expected []model.APMEvent
	addExpectedCount := func(expectedCount int64) {
		expectedEvent := input
		expectedEvent.Transaction = nil
		expectedEvent.Event.Outcome = input.Event.Outcome
		expectedEvent.Processor = model.MetricsetProcessor
		expectedEvent.Metricset = &model.Metricset{
			Name:     "transaction",
			DocCount: expectedCount,
		}
		expectedEvent.Transaction = &model.Transaction{
			Name:   input.Transaction.Name,
			Type:   input.Transaction.Type,
			Result: input.Transaction.Result,
			Root:   input.Parent.ID == "",
			DurationHistogram: model.Histogram{
				Counts: []int64{expectedCount},
				Values: []float64{0},
			},
		}
		expected = append(expected, expectedEvent)
	}
	for _, field := range inputFields {
		for _, value := range []string{"something", "anything"} {
			*field = value
			assert.Zero(t, agg.AggregateTransaction(input))
			assert.Zero(t, agg.AggregateTransaction(input))
			addExpectedCount(2)
		}
	}

	if false {
		// Hostname is complex: if any kubernetes fields are set, then
		// it is taken from Kubernetes.Node.Name, and DetectedHostname
		// is ignored.
		input.Kubernetes.PodName = ""
		for _, value := range []string{"something", "anything"} {
			input.Host.Hostname = value
			assert.Zero(t, agg.AggregateTransaction(input))
			assert.Zero(t, agg.AggregateTransaction(input))
			addExpectedCount(2)
		}

		// Parent.ID only impacts aggregation as far as grouping root and
		// non-root traces.
		for _, value := range []string{"something", "anything"} {
			input.Parent.ID = value
			assert.Zero(t, agg.AggregateTransaction(input))
			assert.Zero(t, agg.AggregateTransaction(input))
		}
		addExpectedCount(4)
	}

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	for _, event := range metricsets {
		event.Metricset.TimeseriesInstanceID = ""
	}
	assert.ElementsMatch(t, expected, metricsets)
}

func BenchmarkAggregateTransaction(b *testing.B) {
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeErrBatchProcessor(nil),
		MaxTransactionGroups:           1000,
		MetricsInterval:                time.Minute,
		HDRHistogramSignificantFigures: 2,
	})
	require.NoError(b, err)

	event := model.APMEvent{
		Processor: model.TransactionProcessor,
		Event:     model.Event{Duration: time.Millisecond},
		Transaction: &model.Transaction{
			Name:                "T-1000",
			RepresentativeCount: 1,
		},
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.AggregateTransaction(event)
		}
	})
}

func makeErrBatchProcessor(err error) model.ProcessBatchFunc {
	return func(context.Context, *model.Batch) error { return err }
}

func makeChanBatchProcessor(ch chan<- model.Batch) model.ProcessBatchFunc {
	return func(ctx context.Context, batch *model.Batch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- *batch:
			return nil
		}
	}
}

func expectBatch(t *testing.T, ch <-chan model.Batch) model.Batch {
	t.Helper()
	select {
	case batch := <-ch:
		return batch
	case <-time.After(time.Second):
		t.Fatal("expected publish")
	}
	panic("unreachable")
}

func batchMetricsets(t testing.TB, batch model.Batch) []model.APMEvent {
	var metricsets []model.APMEvent
	for _, event := range batch {
		if event.Metricset == nil {
			continue
		}
		metricsets = append(metricsets, event)
	}
	return metricsets
}
