// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicetxmetrics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestNewAggregatorConfigInvalid(t *testing.T) {
	report := makeErrBatchProcessor(nil)

	type test struct {
		config AggregatorConfig
		err    string
	}

	for _, test := range []test{{
		config: AggregatorConfig{},
		err:    "BatchProcessor unspecified",
	}, {
		config: AggregatorConfig{
			BatchProcessor: report,
		},
		err: "MaxGroups unspecified or negative",
	}, {
		config: AggregatorConfig{
			BatchProcessor:                 report,
			MaxGroups:                      1,
			HDRHistogramSignificantFigures: 1,
		},
		err: "Interval unspecified or negative",
	}} {
		agg, err := NewAggregator(test.config)
		require.Error(t, err)
		require.Nil(t, agg)
		assert.EqualError(t, err, "invalid aggregator config: "+test.err)
	}
}

func TestAggregatorRun(t *testing.T) {
	batches := make(chan modelpb.Batch, 3)
	config := AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		Interval:                       1 * time.Millisecond,
		RollUpIntervals:                []time.Duration{200 * time.Millisecond, time.Second},
		MaxGroups:                      1000,
		HDRHistogramSignificantFigures: 5,
	}
	agg, err := NewAggregator(config)
	require.NoError(t, err)

	type input struct {
		serviceName         string
		serviceEnvironment  string
		serviceLanguageName string
		agentName           string
		transactionType     string
		outcome             string
		count               float64
	}

	inputs := []input{
		{serviceName: "ignored", agentName: "ignored", transactionType: "ignored", count: 0}, // ignored because count is zero

		{serviceName: "backend", serviceLanguageName: "java", agentName: "java", transactionType: "request", outcome: "success", count: 2.1},
		{serviceName: "backend", serviceLanguageName: "java", agentName: "java", transactionType: "request", outcome: "failure", count: 3},
		{serviceName: "backend", serviceLanguageName: "java", agentName: "java", transactionType: "request", outcome: "unknown", count: 1},

		{serviceName: "backend", serviceLanguageName: "go", agentName: "go", transactionType: "request", outcome: "unknown", count: 1},
		{serviceName: "backend", serviceLanguageName: "go", agentName: "go", transactionType: "background", outcome: "unknown", count: 1},

		{serviceName: "frontend", serviceLanguageName: "js", agentName: "rum-js", transactionType: "page-load", outcome: "unknown", count: 1},
		{serviceName: "frontend", serviceLanguageName: "js", serviceEnvironment: "staging", agentName: "rum-js", transactionType: "page-load", outcome: "unknown", count: 1},
	}

	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			transaction := makeTransaction(
				in.serviceName, in.serviceLanguageName, in.agentName, in.transactionType,
				in.outcome, time.Millisecond, in.count,
			)
			transaction.Service.Environment = in.serviceEnvironment

			batch := modelpb.Batch{transaction}
			err := agg.ProcessBatch(context.Background(), &batch)
			require.NoError(t, err)
			assert.Equal(t, modelpb.Batch{transaction}, batch)
		}(in)
	}
	wg.Wait()

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())
	// Stop the aggregator to ensure all metrics are published.
	assert.NoError(t, agg.Stop(context.Background()))

	for _, interval := range append([]time.Duration{config.Interval}, config.RollUpIntervals...) {
		batch := expectBatch(t, batches)
		metricsets := batchMetricsets(t, batch)
		expected := []*modelpb.APMEvent{{
			Metricset: &modelpb.Metricset{
				Name: "service_transaction", DocCount: 6, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "java"}},
			Agent:   &modelpb.Agent{Name: "java"},
			Transaction: &modelpb.Transaction{
				Type: "request",
				DurationSummary: &modelpb.SummaryMetric{
					Count: 6,
					Sum:   6000, // estimated from histogram
				},
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{1000},
					Counts: []int64{6},
				},
			},
			Event: &modelpb.Event{
				SuccessCount: &modelpb.SummaryMetric{
					Count: 5,
					Sum:   2,
				},
			},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "go"}},
			Agent:   &modelpb.Agent{Name: "go"},
			Transaction: &modelpb.Transaction{
				Type: "request",
				DurationSummary: &modelpb.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "go"}},
			Agent:   &modelpb.Agent{Name: "go"},
			Transaction: &modelpb.Transaction{
				Type: "background",
				DurationSummary: &modelpb.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "frontend", Language: &modelpb.Language{Name: "js"}},
			Agent:   &modelpb.Agent{Name: "rum-js"},
			Transaction: &modelpb.Transaction{
				Type: "page-load",
				DurationSummary: &modelpb.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "frontend", Environment: "staging", Language: &modelpb.Language{Name: "js"}},
			Agent:   &modelpb.Agent{Name: "rum-js"},
			Transaction: &modelpb.Transaction{
				Type: "page-load",
				DurationSummary: &modelpb.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: &modelpb.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}}

		assert.Equal(t, len(expected), len(metricsets))
		sorter := func(events []*modelpb.APMEvent) func(i, j int) bool {
			return func(i, j int) bool {
				e1 := events[i]
				e2 := events[j]
				if e1.Transaction.Type != e2.Transaction.Type {
					return e1.Transaction.Type < e2.Transaction.Type
				}

				if e1.Agent.Name != e2.Agent.Name {
					return e1.Agent.Name < e2.Agent.Name
				}

				return e1.Service.Environment < e2.Service.Environment
			}
		}
		sort.Slice(expected, sorter(expected))
		sort.Slice(metricsets, sorter(metricsets))
		assert.Empty(t, cmp.Diff(expected, metricsets, protocmp.Transform()))
	}

	select {
	case <-batches:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregateTimestamp(t *testing.T) {
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		Interval:                       30 * time.Second,
		MaxGroups:                      1000,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0).UTC()
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		transaction := makeTransaction("service_name", "agent_name", "", "tx_type", "success", 100*time.Millisecond, 1)
		transaction.Timestamp = timestamppb.New(ts)
		batch := modelpb.Batch{transaction}
		err = agg.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
		assert.Empty(t, batchMetricsets(t, batch))
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

func TestAggregatorOverflow(t *testing.T) {
	maxGrps := 4
	overflowCount := 100
	txnDuration := 100 * time.Millisecond
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		Interval:                       10 * time.Second,
		MaxGroups:                      maxGrps,
		HDRHistogramSignificantFigures: 5,
	})
	require.NoError(t, err)

	batch := make(modelpb.Batch, maxGrps+overflowCount) // cause overflow
	for i := 0; i < len(batch); i++ {
		batch[i] = makeTransaction(
			fmt.Sprintf("svc%d", i), "agent", "java", "tx_type", "success", txnDuration, 1,
		)
	}
	go func(t *testing.T) {
		t.Helper()
		require.NoError(t, agg.Run())
	}(t)
	require.NoError(t, agg.ProcessBatch(context.Background(), &batch))
	require.NoError(t, agg.Stop(context.Background()))
	metricsets := batchMetricsets(t, expectBatch(t, batches))
	require.Len(t, metricsets, maxGrps+1) // only one `other` metric should overflow

	// assert monitoring
	registry := monitoring.NewRegistry()
	monitoring.NewFunc(registry, "servicetxmetrics", agg.CollectMonitoring)
	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["servicetxmetrics.active_groups"] = int64(maxGrps)
	expectedMonitoring.Ints["servicetxmetrics.overflowed.total"] = int64(overflowCount)
	assert.Equal(t, expectedMonitoring, monitoring.CollectFlatSnapshot(
		registry, monitoring.Full, false,
	))

	var overflowEvent *modelpb.APMEvent
	for i := range metricsets {
		m := metricsets[i]
		if m.Service.Name == "_other" {
			if overflowEvent != nil {
				require.Fail(t, "only one service should overflow")
			}
			overflowEvent = m
		}
	}
	assert.Empty(t, cmp.Diff(&modelpb.APMEvent{
		Service: &modelpb.Service{
			Name: "_other",
		},
		Transaction: &modelpb.Transaction{
			DurationSummary: &modelpb.SummaryMetric{
				Count: int64(overflowCount),
				Sum:   float64(int64(overflowCount) * txnDuration.Microseconds()),
			},
			DurationHistogram: &modelpb.Histogram{
				Values: []float64{float64(txnDuration.Microseconds())},
				Counts: []int64{int64(overflowCount)},
			},
		},
		Event: &modelpb.Event{
			SuccessCount: &modelpb.SummaryMetric{
				Count: int64(overflowCount),
				Sum:   float64(overflowCount),
			},
		},
		Metricset: &modelpb.Metricset{
			Name:     "service_transaction",
			DocCount: int64(overflowCount),
			Interval: "10s",
			Samples: []*modelpb.MetricsetSample{
				{
					Name:  "service_transaction.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
	}, overflowEvent,
		protocmp.Transform(),
		protocmp.IgnoreFields(&modelpb.APMEvent{}, "timestamp"),
	))
}

func makeTransaction(
	serviceName, serviceLanguageName, agentName, transactionType, outcome string,
	duration time.Duration, count float64,
) *modelpb.APMEvent {
	return &modelpb.APMEvent{
		Agent:   &modelpb.Agent{Name: agentName},
		Service: &modelpb.Service{Name: serviceName, Language: &modelpb.Language{Name: serviceLanguageName}},
		Event: &modelpb.Event{
			Outcome:  outcome,
			Duration: durationpb.New(duration),
		},
		Transaction: &modelpb.Transaction{
			Name:                "transaction_name",
			Type:                transactionType,
			RepresentativeCount: count,
		},
	}
}

func makeErrBatchProcessor(err error) modelpb.BatchProcessor {
	return modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error { return err })
}

func makeChanBatchProcessor(ch chan<- modelpb.Batch) modelpb.BatchProcessor {
	return modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- *batch:
			return nil
		}
	})
}

func expectBatch(t *testing.T, ch <-chan modelpb.Batch) modelpb.Batch {
	t.Helper()
	select {
	case batch := <-ch:
		return batch
	case <-time.After(time.Second * 5):
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
