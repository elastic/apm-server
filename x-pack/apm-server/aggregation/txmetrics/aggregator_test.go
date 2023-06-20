// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package txmetrics_test

import (
	"context"
	"fmt"
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/apm-data/model/modelpb"
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

func TestTxnAggregator_ResetAfterPublish(t *testing.T) {
	batches := make(chan modelpb.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxServices:                    3,
		MaxTransactionGroups:           3,
		MaxTransactionGroupsPerService: 2,
		MetricsInterval:                100 * time.Millisecond,
		HDRHistogramSignificantFigures: 5,
	})
	assert.NoError(t, err)
	batch := modelpb.Batch{
		&modelpb.APMEvent{
			Event: &modelpb.Event{
				Outcome:  "success",
				Duration: durationpb.New(time.Second),
			},
			Transaction: &modelpb.Transaction{
				Type:                "type",
				Name:                "txn1",
				RepresentativeCount: 1,
			},
			Service: &modelpb.Service{Name: "svc1"},
		},
	}
	go func(t *testing.T) {
		t.Helper()
		require.NoError(t, agg.Run())
	}(t)

	registry := monitoring.NewRegistry()
	monitoring.NewFunc(registry, "txmetrics", agg.CollectMonitoring)
	for i := 0; i < 5; i++ {
		// The repition count should be set to be higher than 2 because
		// the aggregators use two datastructures: active and inactive.

		// Each batch has the same transactions configured to not overflow
		require.NoError(t, agg.ProcessBatch(context.Background(), &batch))
		batchMetricsets(t, expectBatch(t, batches))
		expectedMonitoring := monitoring.MakeFlatSnapshot()
		// active_groups is a counter so it will increase for every iteration
		// there should be no expected overflow
		expectedMonitoring.Ints["txmetrics.active_groups"] = int64(i + 1)
		expectedMonitoring.Ints["txmetrics.overflowed.per_service_txn_groups"] = 0
		expectedMonitoring.Ints["txmetrics.overflowed.txn_groups"] = 0
		expectedMonitoring.Ints["txmetrics.overflowed.services"] = 0
		expectedMonitoring.Ints["txmetrics.overflowed.total"] = 0
		assert.Equal(t, expectedMonitoring, monitoring.CollectFlatSnapshot(
			registry,
			monitoring.Full,
			false,
		))
	}
}

func TestTxnAggregatorProcessBatch(t *testing.T) {
	const txnDuration = 100 * time.Millisecond
	for _, tc := range []struct {
		// all unique txns are distributed in unique services sequentially
		// for 7 transactions and 3 services; first three service will receive
		// 2 txns and the last one will receive 1 txn.
		// Note that practically uniqueTxnCount will always be >= uniqueServices.
		name string

		// aggregation limits
		maxServicesLimit        int
		maxTxnGroupsLimit       int
		maxTxnGroupsPerSvcLimit int

		// load distribution of unique transactions across services
		uniqueTxnCount int
		uniqueServices int

		expectedActiveGroups int
		// expectedOverflowReasonPerSvcTxnGrps represent total number of txn groups
		// that overflowed due to per service txn group limit assuming all servies
		// overflow equally. These will be recorded in the `transaction.name: _other`
		// and the corresponding service name documents.
		expectedOverflowReasonPerSvcTxnGrps int
		// expectedOverflowReasonTxnGrps represent total number of txn groups that
		// overflowed due to max txn groups limit. These will be recorded in the
		// `transaction.name: _other` and the corresponding service name documents.
		expectedOverflowReasonTxnGrps int
		// expectedOverflowReasonSvc represents total number of txn groups that
		// overflowed due to max services limit. These will be recorded in the
		// `transaction.name: _other` and the `service.name: _other` document.
		expectedOverflowReasonSvc int
	}{
		{
			name: "no_overflow",

			maxServicesLimit:        10,
			maxTxnGroupsPerSvcLimit: 10,
			maxTxnGroupsLimit:       100,

			uniqueTxnCount: 100,
			uniqueServices: 10,

			expectedActiveGroups:                100,
			expectedOverflowReasonPerSvcTxnGrps: 0,
			expectedOverflowReasonTxnGrps:       0,
			expectedOverflowReasonSvc:           0,
		},
		{
			name: "overflow_for_max_per_svc_txn_grps",

			maxServicesLimit:        20,
			maxTxnGroupsPerSvcLimit: 10,
			maxTxnGroupsLimit:       100,

			uniqueTxnCount: 100,
			uniqueServices: 5,

			expectedActiveGroups:                55, // 10 txn groups + 1 overflow per service
			expectedOverflowReasonPerSvcTxnGrps: 50,
			expectedOverflowReasonTxnGrps:       0,
			expectedOverflowReasonSvc:           0,
		},
		{
			name: "overflow_for_max_txn_grps",

			maxServicesLimit:        20,
			maxTxnGroupsPerSvcLimit: 10,
			maxTxnGroupsLimit:       100,

			uniqueTxnCount: 200,
			uniqueServices: 20,

			expectedActiveGroups:                120,
			expectedOverflowReasonPerSvcTxnGrps: 0,
			expectedOverflowReasonTxnGrps:       100,
			expectedOverflowReasonSvc:           0,
		},
		{
			name: "overflow_for_max_svcs",

			maxServicesLimit:        10,
			maxTxnGroupsPerSvcLimit: 10,
			maxTxnGroupsLimit:       100,

			uniqueTxnCount: 200,
			uniqueServices: 20,

			expectedActiveGroups:                101,
			expectedOverflowReasonPerSvcTxnGrps: 0,
			expectedOverflowReasonTxnGrps:       0,
			expectedOverflowReasonSvc:           100,
		},
		{
			name: "overflow_for_max_svcs_and_max_per_svc_txn_grps",

			maxServicesLimit:        10,
			maxTxnGroupsPerSvcLimit: 10,
			maxTxnGroupsLimit:       100,

			uniqueTxnCount: 400,
			uniqueServices: 20,

			expectedActiveGroups:                111,
			expectedOverflowReasonPerSvcTxnGrps: 100,
			expectedOverflowReasonTxnGrps:       0,
			expectedOverflowReasonSvc:           200,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batches := make(chan modelpb.Batch, 1)
			agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
				BatchProcessor:                 makeChanBatchProcessor(batches),
				MaxServices:                    tc.maxServicesLimit,
				MaxTransactionGroups:           tc.maxTxnGroupsLimit,
				MaxTransactionGroupsPerService: tc.maxTxnGroupsPerSvcLimit,
				MetricsInterval:                30 * time.Second,
				HDRHistogramSignificantFigures: 5,
			})
			require.NoError(t, err)

			repCount := 1
			batch := make(modelpb.Batch, tc.uniqueTxnCount*repCount)
			for i := 0; i < len(batch); i++ {
				batch[i] = &modelpb.APMEvent{
					Event: &modelpb.Event{
						Outcome:  "success",
						Duration: durationpb.New(txnDuration),
					},
					Transaction: &modelpb.Transaction{
						Type:                "type",
						Name:                fmt.Sprintf("foo%d", i%tc.uniqueTxnCount),
						RepresentativeCount: 1,
					},
					Service: &modelpb.Service{Name: fmt.Sprintf("svc%d", i%tc.uniqueServices)},
				}
			}
			go func(t *testing.T) {
				t.Helper()
				require.NoError(t, agg.Run())
			}(t)
			require.NoError(t, agg.ProcessBatch(context.Background(), &batch))
			require.NoError(t, agg.Stop(context.Background()))
			metricsets := batchMetricsets(t, expectBatch(t, batches))
			expectedMonitoring := monitoring.MakeFlatSnapshot()

			expectedMonitoring.Ints["txmetrics.active_groups"] = int64(tc.expectedActiveGroups)
			expectedMonitoring.Ints["txmetrics.overflowed.per_service_txn_groups"] = int64(tc.expectedOverflowReasonPerSvcTxnGrps)
			expectedMonitoring.Ints["txmetrics.overflowed.txn_groups"] = int64(tc.expectedOverflowReasonTxnGrps)
			expectedMonitoring.Ints["txmetrics.overflowed.services"] = int64(tc.expectedOverflowReasonSvc)
			expectedMonitoring.Ints["txmetrics.overflowed.total"] = int64(
				tc.expectedOverflowReasonPerSvcTxnGrps + tc.expectedOverflowReasonTxnGrps + tc.expectedOverflowReasonSvc)
			registry := monitoring.NewRegistry()
			monitoring.NewFunc(registry, "txmetrics", agg.CollectMonitoring)
			assert.Equal(t, expectedMonitoring, monitoring.CollectFlatSnapshot(
				registry,
				monitoring.Full,
				false, // expvar
			))

			var expectedOverflowMetricsets []*modelpb.APMEvent
			var totalOverflowSvcCount int
			totalOverflowIntoAllSvcBuckets := tc.expectedOverflowReasonPerSvcTxnGrps + tc.expectedOverflowReasonTxnGrps
			// Assuming that all services in the test will overflow equally, any overflow due to max
			// transaction groups or per service transaction group limit limit will overflow into the
			// corresponding service's overflow bucket uptil the max services limit.
			if totalOverflowIntoAllSvcBuckets > 0 {
				totalOverflowSvcCount = tc.uniqueServices
				if tc.uniqueServices > tc.maxServicesLimit {
					totalOverflowSvcCount = tc.maxServicesLimit
				}
			}
			// If there are any overflows due to the max services limit then the overflow
			// will be aggregated under a special `service.name: _other` bucket.
			if tc.expectedOverflowReasonSvc > 0 {
				expectedOverflowMetricsets = append(
					expectedOverflowMetricsets,
					createOverflowMetricset(tc.expectedOverflowReasonSvc, repCount, txnDuration),
				)
			}
			for i := 0; i < totalOverflowSvcCount; i++ {
				totalOverflowForEachSvcBuckets := totalOverflowIntoAllSvcBuckets / totalOverflowSvcCount
				expectedOverflowMetricsets = append(
					expectedOverflowMetricsets,
					createOverflowMetricset(totalOverflowForEachSvcBuckets, repCount, txnDuration),
				)
			}
			var expectedMetrics []*modelpb.APMEvent
			var finalMetricset []*modelpb.APMEvent
			for _, v := range metricsets {
				if v.Transaction.Name == "_other" {
					finalMetricset = append(finalMetricset, v)
				}
			}
			for _, a := range expectedOverflowMetricsets {
				if a.Transaction.Name == "_other" {
					expectedMetrics = append(expectedMetrics, a)
				}
			}
			assert.Empty(t, cmp.Diff(
				expectedMetrics,
				finalMetricset,
				protocmp.Transform(),
				protocmp.IgnoreFields(&modelpb.APMEvent{}, "timestamp"),
				protocmp.IgnoreFields(&modelpb.Service{}, "name"),
				cmpopts.SortSlices(func(a, b *modelpb.APMEvent) bool {
					return a.GetService().GetName() < b.GetService().GetName()
				}),
			))
		})
	}
}

func TestAggregatorRun(t *testing.T) {
	batches := make(chan modelpb.Batch, 6)
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
	now := time.Now().UTC()
	for i := 0; i < 1000; i++ {
		event := modelpb.APMEvent{
			Event:     &modelpb.Event{Duration: durationpb.New(time.Second)},
			Timestamp: timestamppb.New(now),
			Labels: modelpb.Labels{
				"department_name": &modelpb.LabelValue{Global: true, Value: "apm"},
				"organization":    &modelpb.LabelValue{Global: true, Value: "observability"},
				"company":         &modelpb.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: modelpb.NumericLabels{
				"user_id":     &modelpb.NumericLabelValue{Global: true, Value: 100},
				"cost_center": &modelpb.NumericLabelValue{Global: true, Value: 10},
			},
			Transaction: &modelpb.Transaction{
				Type:                "type",
				Name:                "T-1000",
				RepresentativeCount: 1,
			},
		}
		if i%2 == 0 {
			event.Event = &modelpb.Event{Duration: durationpb.New(100 * time.Millisecond)}
		}
		agg.AggregateTransaction(&event)
	}
	for i := 0; i < 800; i++ {
		event := modelpb.APMEvent{
			Event:     &modelpb.Event{Duration: durationpb.New(time.Second)},
			Timestamp: timestamppb.New(now),
			Transaction: &modelpb.Transaction{
				Type:                "type",
				Name:                "T-800",
				RepresentativeCount: 2.5,
			},
		}
		if i%2 == 0 {
			event.Event = &modelpb.Event{Duration: durationpb.New(100 * time.Millisecond)}
		}
		agg.AggregateTransaction(&event)
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
		assert.Equal(t, modelpb.Labels{
			"department_name": &modelpb.LabelValue{Value: "apm"},
			"organization":    &modelpb.LabelValue{Value: "observability"},
			"company":         &modelpb.LabelValue{Value: "elastic"},
		}, modelpb.Labels(metricsets[0].Labels))
		assert.Equal(t, modelpb.NumericLabels{
			"user_id":     &modelpb.NumericLabelValue{Value: 100},
			"cost_center": &modelpb.NumericLabelValue{Value: 10},
		}, modelpb.NumericLabels(metricsets[0].NumericLabels))
		assert.Equal(t, []int64{500, 500}, metricsets[0].Transaction.DurationHistogram.Counts)
		assert.Equal(t, "T-800", metricsets[1].Transaction.Name)
		assert.Empty(t, metricsets[1].Labels)
		assert.Empty(t, metricsets[1].NumericLabels)
		assert.Equal(t, []int64{1000, 1000}, metricsets[1].Transaction.DurationHistogram.Counts)
		for _, event := range metricsets {
			actual := event.Timestamp.AsTime()
			if event.Timestamp == nil {
				actual = time.Time{}
			}
			assert.Equal(t, now.Truncate(intervals[i]), actual)
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
	batches := make(chan modelpb.Batch, 1)
	chanBatchProcessor := makeChanBatchProcessor(batches)
	processBatchErr := errors.New("report failed")
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
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
		agg.AggregateTransaction(&modelpb.APMEvent{
			Transaction: &modelpb.Transaction{
				Type:                "type",
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
			batches := make(chan modelpb.Batch, 1)
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
				agg.AggregateTransaction(&modelpb.APMEvent{
					Transaction: &modelpb.Transaction{
						Type:                "type",
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
	batches := make(chan modelpb.Batch, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		MaxTransactionGroups:           2,
		MaxTransactionGroupsPerService: 2,
		MaxServices:                    2,
		MetricsInterval:                30 * time.Second,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0).UTC()
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		agg.AggregateTransaction(&modelpb.APMEvent{
			Timestamp: timestamppb.New(ts),
			Transaction: &modelpb.Transaction{
				Type:                "type",
				Name:                "name",
				RepresentativeCount: 1,
			},
		})
	}

	go agg.Run()
	err = agg.Stop(context.Background()) // stop to flush
	require.NoError(t, err)

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	require.Len(t, metricsets, 2)
	sort.Slice(metricsets, func(i, j int) bool {
		return metricsets[i].Timestamp.AsTime().Before(metricsets[j].Timestamp.AsTime())
	})
	assert.Equal(t, t0, metricsets[0].Timestamp.AsTime())
	assert.Equal(t, t0.Add(30*time.Second), metricsets[1].Timestamp.AsTime())
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
		batches := make(chan modelpb.Batch, 1)
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
			agg.AggregateTransaction(&modelpb.APMEvent{
				Event: &modelpb.Event{Duration: durationpb.New(duration)},
				Transaction: &modelpb.Transaction{
					Type:                "type",
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
	t.Skip("TODO FIX")
	batches := make(chan modelpb.Batch, 1)
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

	input := modelpb.APMEvent{
		Transaction: &modelpb.Transaction{
			Type:                "type",
			RepresentativeCount: 1,
		},
		Event:      &modelpb.Event{},
		Agent:      &modelpb.Agent{},
		Service:    &modelpb.Service{Node: &modelpb.ServiceNode{}, Language: &modelpb.Language{}, Runtime: &modelpb.Runtime{}},
		Container:  &modelpb.Container{},
		Kubernetes: &modelpb.Kubernetes{},
		Cloud:      &modelpb.Cloud{},
		Host:       &modelpb.Host{Os: &modelpb.OS{}},
		Faas:       &modelpb.Faas{},
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
		&input.Container.Id,
		&input.Kubernetes.PodName,
		&input.Cloud.Provider,
		&input.Cloud.Region,
		&input.Cloud.AvailabilityZone,
		&input.Cloud.AccountId,
		&input.Cloud.AccountName,
		&input.Cloud.ProjectId,
		&input.Cloud.ProjectName,
		&input.Cloud.MachineType,
		&input.Cloud.ServiceName,
		&input.Service.Language.Name,
		&input.Service.Language.Version,
		&input.Service.Runtime.Name,
		&input.Service.Runtime.Version,
		&input.Host.Os.Platform,
		&input.Faas.Id,
		&input.Faas.TriggerType,
		&input.Faas.Name,
		&input.Faas.Version,
	}
	boolPtrInputFields := []**bool{
		&input.Faas.ColdStart,
	}

	var expected []*modelpb.APMEvent
	addExpectedCount := func(expectedCount int64) {
		expectedEvent := proto.Clone(&input).(*modelpb.APMEvent)
		expectedEvent.Transaction = &modelpb.Transaction{
			Name:   input.Transaction.Name,
			Type:   input.Transaction.Type,
			Result: input.Transaction.Result,
			Root:   input.GetParentId() == "",
			DurationHistogram: &modelpb.Histogram{
				Counts: []int64{expectedCount},
				Values: []float64{0},
			},
			DurationSummary: &modelpb.SummaryMetric{
				Count: expectedCount,
				Sum:   0,
			},
		}
		expectedEvent.Event.Outcome = input.Event.Outcome
		expectedEvent.Metricset = &modelpb.Metricset{
			Name:     "transaction",
			DocCount: expectedCount,
			Interval: "0s",
		}
		expected = append(expected, expectedEvent)
	}
	for _, field := range inputFields {
		for _, value := range []string{"something", "anything"} {
			*field = value
			agg.AggregateTransaction(&input)
			agg.AggregateTransaction(&input)
			addExpectedCount(2)
		}
	}
	for _, field := range boolPtrInputFields {
		for _, value := range []bool{false, true} {
			value := value
			*field = &value
			agg.AggregateTransaction(&input)
			agg.AggregateTransaction(&input)
			addExpectedCount(2)
		}
		*field = nil
	}

	if false {
		// Hostname is complex: if any kubernetes fields are set, then
		// it is taken from Kubernetes.Node.Name, and DetectedHostname
		// is ignored.
		input.Kubernetes.PodName = ""
		for _, value := range []string{"something", "anything"} {
			input.Host.Hostname = value
			agg.AggregateTransaction(&input)
			agg.AggregateTransaction(&input)
			addExpectedCount(2)
		}

		// Parent.ID only impacts aggregation as far as grouping root and
		// non-root traces.
		for _, value := range []string{"something", "anything"} {
			input.ParentId = value
			agg.AggregateTransaction(&input)
			agg.AggregateTransaction(&input)
		}
		addExpectedCount(4)
	}

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	assert.Empty(t, cmp.Diff(expected, metricsets,
		protocmp.Transform(),
		protocmp.IgnoreEmptyMessages(),
		cmpopts.SortSlices(func(x, y *modelpb.APMEvent) bool {
			if x.GetTransaction().GetName() != y.GetTransaction().GetName() {
				return x.GetTransaction().GetName() < y.GetTransaction().GetName()
			}
			if x.GetEvent().GetOutcome() != y.GetEvent().GetOutcome() {
				return x.GetEvent().GetOutcome() < y.GetEvent().GetOutcome()
			}
			return x.GetAgent().GetName() < y.GetAgent().GetName()
		}),
	))
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

	event := modelpb.APMEvent{
		Event: &modelpb.Event{Duration: durationpb.New(time.Millisecond)},
		Transaction: &modelpb.Transaction{
			Type:                "type",
			Name:                "T-1000",
			RepresentativeCount: 1,
		},
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.AggregateTransaction(&event)
		}
	})
}

func makeErrBatchProcessor(err error) modelpb.ProcessBatchFunc {
	return func(context.Context, *modelpb.Batch) error { return err }
}

func makeChanBatchProcessor(ch chan<- modelpb.Batch) modelpb.ProcessBatchFunc {
	return func(ctx context.Context, batch *modelpb.Batch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- *batch:
			return nil
		}
	}
}

func expectBatch(t *testing.T, ch <-chan modelpb.Batch) modelpb.Batch {
	t.Helper()
	select {
	case batch := <-ch:
		return batch
	case <-time.After(time.Second):
		t.Fatal("expected publish")
	}
	panic("unreachable")
}

func batchMetricsets(t testing.TB, batch modelpb.Batch) []*modelpb.APMEvent {
	var metricsets []*modelpb.APMEvent
	for _, event := range batch {
		if event.Metricset == nil {
			continue
		}
		metricsets = append(metricsets, event)
	}
	return metricsets
}

func createOverflowMetricset(overflowCount, repCount int, txnDuration time.Duration) *modelpb.APMEvent {
	return &modelpb.APMEvent{
		Service: &modelpb.Service{},
		Transaction: &modelpb.Transaction{
			Name: "_other",
			DurationHistogram: &modelpb.Histogram{
				Counts: []int64{int64(overflowCount * repCount)},
				Values: []float64{float64(txnDuration.Microseconds())},
			},
			DurationSummary: &modelpb.SummaryMetric{
				Count: int64(overflowCount * repCount),
				Sum:   float64(time.Duration(float64(overflowCount*repCount) * float64(txnDuration)).Microseconds()),
			},
		},
		Metricset: &modelpb.Metricset{
			Name:     "transaction",
			DocCount: int64(overflowCount * repCount),
			Interval: "30s",
			Samples: []*modelpb.MetricsetSample{
				{
					Name:  "transaction.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
	}
}
