// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicetxmetrics

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model"
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
	batches := make(chan model.Batch, 3)
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
		serviceName        string
		serviceEnvironment string
		agentName          string
		transactionType    string
		outcome            string
		count              float64
	}

	inputs := []input{
		{serviceName: "ignored", agentName: "ignored", transactionType: "ignored", count: 0}, // ignored because count is zero

		{serviceName: "backend", agentName: "java", transactionType: "request", outcome: "success", count: 2},
		{serviceName: "backend", agentName: "java", transactionType: "request", outcome: "failure", count: 3},
		{serviceName: "backend", agentName: "java", transactionType: "request", outcome: "unknown", count: 1},

		{serviceName: "backend", agentName: "go", transactionType: "request", outcome: "unknown", count: 1},
		{serviceName: "backend", agentName: "go", transactionType: "background", outcome: "unknown", count: 1},

		{serviceName: "frontend", agentName: "rum-js", transactionType: "page-load", outcome: "unknown", count: 1},
		{serviceName: "frontend", serviceEnvironment: "staging", agentName: "rum-js", transactionType: "page-load", outcome: "unknown", count: 1},
	}

	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			transaction := makeTransaction(
				in.serviceName, in.agentName, in.transactionType,
				in.outcome, time.Millisecond, in.count,
			)
			transaction.Service.Environment = in.serviceEnvironment

			batch := model.Batch{transaction}
			err := agg.ProcessBatch(context.Background(), &batch)
			require.NoError(t, err)
			assert.Equal(t, model.Batch{transaction}, batch)
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
		expected := []model.APMEvent{{
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_transaction", DocCount: 6, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend"},
			Agent:   model.Agent{Name: "java"},
			Transaction: &model.Transaction{
				Type: "request",
				DurationSummary: model.SummaryMetric{
					Count: 6,
					Sum:   6000, // 6ms in micros
				},
				DurationHistogram: model.Histogram{
					Values: []float64{1000, 2000, 3000},
					Counts: []int64{1, 2, 3},
				},
			},
			Event: model.Event{
				SuccessCount: model.SummaryMetric{
					Count: 5,
					Sum:   2,
				},
			},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend"},
			Agent:   model.Agent{Name: "go"},
			Transaction: &model.Transaction{
				Type: "request",
				DurationSummary: model.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: model.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend"},
			Agent:   model.Agent{Name: "go"},
			Transaction: &model.Transaction{
				Type: "background",
				DurationSummary: model.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: model.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "frontend"},
			Agent:   model.Agent{Name: "rum-js"},
			Transaction: &model.Transaction{
				Type: "page-load",
				DurationSummary: model.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: model.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_transaction", DocCount: 1, Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "frontend", Environment: "staging"},
			Agent:   model.Agent{Name: "rum-js"},
			Transaction: &model.Transaction{
				Type: "page-load",
				DurationSummary: model.SummaryMetric{
					Count: 1,
					Sum:   1000, // 1ms in micros
				},
				DurationHistogram: model.Histogram{
					Values: []float64{1000},
					Counts: []int64{1},
				},
			},
		}}

		assert.Equal(t, len(expected), len(metricsets))
		out := cmp.Diff(expected, metricsets, cmpopts.IgnoreTypes(netip.Addr{}), cmpopts.SortSlices(func(e1 model.APMEvent, e2 model.APMEvent) bool {
			if e1.Transaction.Type != e2.Transaction.Type {
				return e1.Transaction.Type < e2.Transaction.Type
			}

			if e1.Agent.Name != e2.Agent.Name {
				return e1.Agent.Name < e2.Agent.Name
			}

			return e1.Service.Environment < e2.Service.Environment
		}))
		assert.Empty(t, out)
	}

	select {
	case <-batches:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregateTimestamp(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		Interval:                       30 * time.Second,
		MaxGroups:                      1000,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0)
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		transaction := makeTransaction("service_name", "agent_name", "tx_type", "success", 100*time.Millisecond, 1)
		transaction.Timestamp = ts
		batch := model.Batch{transaction}
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
		return metricsets[i].Timestamp.Before(metricsets[j].Timestamp)
	})
	assert.Equal(t, t0, metricsets[0].Timestamp)
	assert.Equal(t, t0.Add(30*time.Second), metricsets[1].Timestamp)
}

func TestAggregatorOverflow(t *testing.T) {
	maxGrps := 4
	overflowCount := 100
	txnDuration := 100 * time.Millisecond
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor:                 makeChanBatchProcessor(batches),
		Interval:                       10 * time.Second,
		MaxGroups:                      maxGrps,
		HDRHistogramSignificantFigures: 5,
	})
	require.NoError(t, err)

	batch := make(model.Batch, maxGrps+overflowCount) // cause overflow
	for i := 0; i < len(batch); i++ {
		batch[i] = makeTransaction(
			fmt.Sprintf("svc%d", i), "agent", "tx_type", "success", txnDuration, 1,
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
	var overflowEvent *model.APMEvent
	for i := range metricsets {
		m := metricsets[i]
		if m.Service.Name == "_other" {
			if overflowEvent != nil {
				require.Fail(t, "only one service should overflow")
			}
			overflowEvent = &m
		}
	}
	assert.Empty(t, cmp.Diff(model.APMEvent{
		Service: model.Service{
			Name: "_other",
		},
		Processor: model.MetricsetProcessor,
		Transaction: &model.Transaction{
			DurationSummary: model.SummaryMetric{
				Count: int64(overflowCount),
				Sum:   float64(int64(overflowCount) * txnDuration.Microseconds()),
			},
			DurationHistogram: model.Histogram{
				Values: []float64{float64(txnDuration.Microseconds())},
				Counts: []int64{int64(overflowCount)},
			},
		},
		Event: model.Event{
			SuccessCount: model.SummaryMetric{
				Count: int64(overflowCount),
				Sum:   float64(overflowCount),
			},
		},
		Metricset: &model.Metricset{
			Name:     "service_transaction",
			DocCount: int64(overflowCount),
			Interval: "10s",
			Samples: []model.MetricsetSample{
				{
					Name:  "service_transaction.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
	}, *overflowEvent, cmpopts.IgnoreTypes(netip.Addr{}, time.Time{})))
}

func makeTransaction(
	serviceName, agentName, transactionType, outcome string,
	duration time.Duration, count float64,
) model.APMEvent {
	return model.APMEvent{
		Agent:   model.Agent{Name: agentName},
		Service: model.Service{Name: serviceName},
		Event: model.Event{
			Outcome:  outcome,
			Duration: duration,
		},
		Processor: model.TransactionProcessor,
		Transaction: &model.Transaction{
			Name:                "transaction_name",
			Type:                transactionType,
			RepresentativeCount: count,
		},
	}
}

func makeErrBatchProcessor(err error) model.BatchProcessor {
	return model.ProcessBatchFunc(func(context.Context, *model.Batch) error { return err })
}

func makeChanBatchProcessor(ch chan<- model.Batch) model.BatchProcessor {
	return model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- *batch:
			return nil
		}
	})
}

func expectBatch(t *testing.T, ch <-chan model.Batch) model.Batch {
	t.Helper()
	select {
	case batch := <-ch:
		return batch
	case <-time.After(time.Second * 5):
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
