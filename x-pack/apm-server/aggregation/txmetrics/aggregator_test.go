// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package txmetrics_test

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
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
		err: "MaxTransactionGroupsPerService unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor:                 batchProcessor,
			MaxTransactionGroups:           1,
			MaxTransactionGroupsPerService: 1,
		},
		err: "MaxServices unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor:                 batchProcessor,
			MaxTransactionGroups:           1,
			MaxTransactionGroupsPerService: 1,
			MaxServices:                    1,
			HDRHistogramSignificantFigures: 5,
		},
		err: "Interval unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			BatchProcessor:                 batchProcessor,
			MaxTransactionGroups:           1,
			MaxTransactionGroupsPerService: 1,
			MaxServices:                    1,
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

func TestTxnAggregatorProcessBatch(t *testing.T) {
	const maxSvcs = 20
	const maxTxnGrps = 20
	const maxTxnGrpsPerSvc = 2
	const txnDuration = 100 * time.Millisecond
	for _, tc := range []struct {
		// all unique txns are distributed in unique services sequentially
		// for 7 transactions and 3 services; first three service will receive
		// 2 txns and the last one will receive 1 txn.
		// Note that practically uniqueTxnCount will always be >= uniqueServices.
		name                             string
		uniqueTxnCount                   int
		uniqueServices                   int
		expectedActiveGroups             int64
		expectedPerSvcTxnLimitOverflow   int
		expectedOtherSvcTxnLimitOverflow int // we will design tests to overflow all the services equally
		expectedTotalOverflow            int64
	}{
		{
			name:                             "record_into_other_txn_if_txn_per_svcs_limit_breached",
			uniqueTxnCount:                   20,
			uniqueServices:                   2,
			expectedActiveGroups:             6,
			expectedPerSvcTxnLimitOverflow:   8,
			expectedOtherSvcTxnLimitOverflow: 0,
			expectedTotalOverflow:            16,
		},
		{
			name:                             "record_into_other_txn_if_txn_grps_limit_breached",
			uniqueTxnCount:                   60,
			uniqueServices:                   20,
			expectedActiveGroups:             40,
			expectedPerSvcTxnLimitOverflow:   2,
			expectedOtherSvcTxnLimitOverflow: 0,
			expectedTotalOverflow:            40,
		},
		{
			name:                             "record_into_other_txn_other_svc_if_txn_grps_and_svcs_limit_breached",
			uniqueTxnCount:                   60,
			uniqueServices:                   60,
			expectedActiveGroups:             21,
			expectedPerSvcTxnLimitOverflow:   0,
			expectedOtherSvcTxnLimitOverflow: 40,
			expectedTotalOverflow:            40,
		},
		{
			name:                             "all_overflow",
			uniqueTxnCount:                   600,
			uniqueServices:                   60,
			expectedActiveGroups:             41,
			expectedPerSvcTxnLimitOverflow:   9,
			expectedOtherSvcTxnLimitOverflow: 400,
			expectedTotalOverflow:            580,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batches := make(chan model.Batch, 1)
			agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
				BatchProcessor:                 makeChanBatchProcessor(batches),
				MaxServices:                    maxSvcs,
				MaxTransactionGroups:           maxTxnGrps,
				MaxTransactionGroupsPerService: maxTxnGrpsPerSvc,
				MetricsInterval:                30 * time.Second,
				HDRHistogramSignificantFigures: 5,
			})
			require.NoError(t, err)

			repCount := 5
			batch := make(model.Batch, tc.uniqueTxnCount*repCount)
			for i := 0; i < len(batch); i++ {
				batch[i] = model.APMEvent{
					Processor: model.TransactionProcessor,
					Event: model.Event{
						Outcome:  "success",
						Duration: txnDuration,
					},
					Transaction: &model.Transaction{
						Name:                fmt.Sprintf("foo%d", i%tc.uniqueTxnCount),
						RepresentativeCount: 1,
					},
					Service: model.Service{Name: fmt.Sprintf("svc%d", i%tc.uniqueServices)},
				}
			}
			go func(t *testing.T) {
				t.Helper()
				require.NoError(t, agg.Run())
			}(t)
			require.NoError(t, agg.ProcessBatch(context.Background(), &batch))

			expectedMonitoring := monitoring.MakeFlatSnapshot()
			expectedMonitoring.Ints["txmetrics.active_groups"] = tc.expectedActiveGroups
			expectedMonitoring.Ints["txmetrics.overflowed"] = tc.expectedTotalOverflow * int64(repCount)
			registry := monitoring.NewRegistry()
			monitoring.NewFunc(registry, "txmetrics", agg.CollectMonitoring)
			assert.Equal(t, expectedMonitoring, monitoring.CollectFlatSnapshot(
				registry,
				monitoring.Full,
				false, // expvar
			))

			require.NoError(t, agg.Stop(context.Background()))
			metricsets := batchMetricsets(t, expectBatch(t, batches))
			var expectedOverflowMetricsets []model.APMEvent
			var totalOverflowSvcCount int
			if tc.expectedPerSvcTxnLimitOverflow > 0 {
				totalOverflowSvcCount = tc.uniqueServices
				if tc.uniqueServices > maxSvcs {
					totalOverflowSvcCount = maxSvcs
				}
			}
			if tc.expectedOtherSvcTxnLimitOverflow > 0 {
				expectedOverflowMetricsets = append(
					expectedOverflowMetricsets,
					createOverflowMetricset(tc.expectedOtherSvcTxnLimitOverflow, repCount, txnDuration),
				)
			}
			for i := 0; i < totalOverflowSvcCount; i++ {
				expectedOverflowMetricsets = append(
					expectedOverflowMetricsets,
					createOverflowMetricset(tc.expectedPerSvcTxnLimitOverflow, repCount, txnDuration),
				)
			}
			assert.Empty(t, cmp.Diff(
				expectedOverflowMetricsets,
				metricsets,
				cmpopts.IgnoreSliceElements(func(a model.APMEvent) bool {
					return a.Transaction.Name != "other"
				}),
				cmpopts.IgnoreTypes(netip.Addr{}),
				cmpopts.IgnoreFields(model.APMEvent{}, "Timestamp", "Service.Name"),
				cmpopts.SortSlices(func(a, b model.APMEvent) bool {
					return a.Service.Name < b.Service.Name
				}),
			))
		})
	}
}

func TestAggregatorRun(t *testing.T) {
	batches := make(chan model.Batch, 6)
	config := txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MaxTransactionGroupsPerService: 2,
		MaxServices:                    2,
		MetricsInterval:                10 * time.Millisecond,
		RollUpIntervals:                []time.Duration{200 * time.Millisecond, time.Second},
		HDRHistogramSignificantFigures: 1,
	}
	agg, err := txmetrics.NewAggregator(config)
	require.NoError(t, err)

	intervals := append([]time.Duration{config.MetricsInterval}, config.RollUpIntervals...)
	now := time.Now()
	for i := 0; i < 1000; i++ {
		event := model.APMEvent{
			Event:     model.Event{Duration: time.Second},
			Timestamp: now,
			Processor: model.TransactionProcessor,
			Labels: model.Labels{
				"department_name": model.LabelValue{Global: true, Value: "apm"},
				"organization":    model.LabelValue{Global: true, Value: "observability"},
				"company":         model.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: model.NumericLabels{
				"user_id":     model.NumericLabelValue{Global: true, Value: 100},
				"cost_center": model.NumericLabelValue{Global: true, Value: 10},
			},
			Transaction: &model.Transaction{
				Name:                "T-1000",
				RepresentativeCount: 1,
			},
		}
		if i%2 == 0 {
			event.Event = model.Event{Duration: 100 * time.Millisecond}
		}
		agg.AggregateTransaction(event)
	}
	for i := 0; i < 800; i++ {
		event := model.APMEvent{
			Event:     model.Event{Duration: time.Second},
			Timestamp: now,
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "T-800",
				RepresentativeCount: 1,
			},
		}
		if i%2 == 0 {
			event.Event = model.Event{Duration: 100 * time.Millisecond}
		}
		agg.AggregateTransaction(event)
	}

	go agg.Run()
	defer agg.Stop(context.Background())
	// Stop the aggregator to ensure all metrics are published.
	assert.NoError(t, agg.Stop(context.Background()))

	for i := 0; i < 3; i++ {
		batch := expectBatch(t, batches)
		metricsets := batchMetricsets(t, batch)
		require.Len(t, metricsets, 2)
		sort.Slice(metricsets, func(i, j int) bool {
			return metricsets[i].Transaction.Name < metricsets[j].Transaction.Name
		})

		assert.Equal(t, "T-1000", metricsets[0].Transaction.Name)
		assert.Equal(t, model.Labels{
			"department_name": model.LabelValue{Value: "apm"},
			"organization":    model.LabelValue{Value: "observability"},
			"company":         model.LabelValue{Value: "elastic"},
		}, metricsets[0].Labels)
		assert.Equal(t, model.NumericLabels{
			"user_id":     model.NumericLabelValue{Value: 100},
			"cost_center": model.NumericLabelValue{Value: 10},
		}, metricsets[0].NumericLabels)
		assert.Equal(t, []int64{500, 500}, metricsets[0].Transaction.DurationHistogram.Counts)
		assert.Equal(t, "T-800", metricsets[1].Transaction.Name)
		assert.Empty(t, metricsets[1].Labels)
		assert.Empty(t, metricsets[1].NumericLabels)
		assert.Equal(t, []int64{400, 400}, metricsets[1].Transaction.DurationHistogram.Counts)
		for _, event := range metricsets {
			assert.Equal(t, now.Truncate(intervals[i]), event.Timestamp)
			assert.Equal(t, fmt.Sprintf("%.0fs", intervals[i].Seconds()), event.Metricset.Interval)
		}
	}

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
		MaxTransactionGroupsPerService: 1,
		MaxServices:                    2,
		MetricsInterval:                10 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
		Logger:                         logger,
	})
	require.NoError(t, err)

	go agg.Run()
	defer agg.Stop(context.Background())

	for i := 0; i < 2; i++ {
		agg.AggregateTransaction(model.APMEvent{
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "T-1000",
				RepresentativeCount: 1,
			},
		})
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
	for _, tc := range []struct {
		name                 string
		representativeCounts []float64
		expectedCount        int64
	}{
		{
			name:                 "int",
			representativeCounts: []float64{2},
			expectedCount:        2,
		},
		{
			name:                 "float",
			representativeCounts: []float64{1.50},
			expectedCount:        2,
		},
		{
			name:                 "mix",
			representativeCounts: []float64{1, 1.5},
			expectedCount:        3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batches := make(chan model.Batch, 1)
			agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
				BatchProcessor:                 makeChanBatchProcessor(batches),
				MaxTransactionGroups:           1,
				MaxTransactionGroupsPerService: 1,
				MaxServices:                    1,
				MetricsInterval:                time.Microsecond,
				HDRHistogramSignificantFigures: 1,
			})
			require.NoError(t, err)

			for _, rc := range tc.representativeCounts {
				agg.AggregateTransaction(model.APMEvent{
					Processor: model.TransactionProcessor,
					Transaction: &model.Transaction{
						Name:                "foo",
						RepresentativeCount: rc,
					},
				})
			}

			go agg.Run()
			require.NoError(t, agg.Stop(context.Background()))

			batch := expectBatch(t, batches)
			metricsets := batchMetricsets(t, batch)
			require.Len(t, metricsets, 1)
			require.Nil(t, metricsets[0].Metricset.Samples)
			require.NotNil(t, metricsets[0].Transaction)
			durationHistogram := metricsets[0].Transaction.DurationHistogram
			assert.Equal(t, []int64{tc.expectedCount}, durationHistogram.Counts)
		})
	}
}

func TestAggregateTimestamp(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MaxTransactionGroupsPerService: 2,
		MaxServices:                    2,
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
			MaxTransactionGroupsPerService: 1,
			MaxServices:                    2,
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
			agg.AggregateTransaction(model.APMEvent{
				Processor: model.TransactionProcessor,
				Event:     model.Event{Duration: duration},
				Transaction: &model.Transaction{
					Name:                "T-1000",
					RepresentativeCount: 1,
				},
			})
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
		MaxTransactionGroupsPerService: 100,
		MaxServices:                    3,
		MetricsInterval:                100 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)
	go agg.Run()
	defer agg.Stop(context.Background())

	falseP := false
	input := model.APMEvent{
		Processor:   model.TransactionProcessor,
		Transaction: &model.Transaction{RepresentativeCount: 1},
	}
	input.FAAS.Coldstart = &falseP
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
		&input.Cloud.AccountID,
		&input.Cloud.AccountName,
		&input.Cloud.ProjectID,
		&input.Cloud.ProjectName,
		&input.Cloud.MachineType,
		&input.Cloud.ServiceName,
		&input.Service.Language.Name,
		&input.Service.Language.Version,
		&input.Service.Runtime.Name,
		&input.Service.Runtime.Version,
		&input.Host.OS.Platform,
		&input.FAAS.ID,
		&input.FAAS.TriggerType,
		&input.FAAS.Name,
		&input.FAAS.Version,
	}
	boolInputFields := []*bool{
		input.FAAS.Coldstart,
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
			Interval: "0s",
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
			DurationSummary: model.SummaryMetric{
				Count: expectedCount,
				Sum:   0,
			},
		}
		expected = append(expected, expectedEvent)
	}
	for _, field := range inputFields {
		for _, value := range []string{"something", "anything"} {
			*field = value
			agg.AggregateTransaction(input)
			agg.AggregateTransaction(input)
			addExpectedCount(2)
		}
	}
	for _, field := range boolInputFields {
		*field = true
		agg.AggregateTransaction(input)
		agg.AggregateTransaction(input)
		addExpectedCount(2)
	}

	if false {
		// Hostname is complex: if any kubernetes fields are set, then
		// it is taken from Kubernetes.Node.Name, and DetectedHostname
		// is ignored.
		input.Kubernetes.PodName = ""
		for _, value := range []string{"something", "anything"} {
			input.Host.Hostname = value
			agg.AggregateTransaction(input)
			agg.AggregateTransaction(input)
			addExpectedCount(2)
		}

		// Parent.ID only impacts aggregation as far as grouping root and
		// non-root traces.
		for _, value := range []string{"something", "anything"} {
			input.Parent.ID = value
			agg.AggregateTransaction(input)
			agg.AggregateTransaction(input)
		}
		addExpectedCount(4)
	}

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	assert.ElementsMatch(t, expected, metricsets)
}

func BenchmarkAggregateTransaction(b *testing.B) {
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeErrBatchProcessor(nil),
		MaxTransactionGroups:           1000,
		MaxTransactionGroupsPerService: 100,
		MaxServices:                    1000,
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

func createOverflowMetricset(overflowCount, repCount int, txnDuration time.Duration) model.APMEvent {
	return model.APMEvent{
		Processor: model.MetricsetProcessor,
		Transaction: &model.Transaction{
			Name: "other",
			DurationHistogram: model.Histogram{
				Counts: []int64{int64(overflowCount * repCount)},
				Values: []float64{float64(txnDuration.Microseconds())},
			},
			DurationSummary: model.SummaryMetric{
				Count: int64(overflowCount * repCount),
				Sum:   float64(txnDuration.Microseconds()),
			},
		},
		Metricset: &model.Metricset{
			Name:     "transaction",
			DocCount: int64(overflowCount * repCount),
			Interval: "30s",
			Samples: []model.MetricsetSample{
				{
					Name:  "transaction.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
	}
}
