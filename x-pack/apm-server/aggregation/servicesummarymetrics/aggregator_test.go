// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicesummarymetrics

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

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-data/model"
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
			BatchProcessor: report,
			MaxGroups:      1,
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
		BatchProcessor:  makeChanBatchProcessor(batches),
		Interval:        1 * time.Millisecond,
		RollUpIntervals: []time.Duration{200 * time.Millisecond, time.Second},
		MaxGroups:       1000,
	}
	agg, err := NewAggregator(config)
	require.NoError(t, err)

	apmEvents := []model.APMEvent{
		{
			Agent:   model.Agent{Name: "java"},
			Service: model.Service{Name: "backend", Language: model.Language{Name: "java"}},
			Event: model.Event{
				Outcome:  "success",
				Duration: time.Millisecond,
			},
			Processor: model.TransactionProcessor,
			Transaction: &model.Transaction{
				Name:                "transaction_name",
				Type:                "request",
				RepresentativeCount: 1,
			},
			Labels: model.Labels{
				"department_name": model.LabelValue{Global: true, Value: "apm"},
				"organization":    model.LabelValue{Global: true, Value: "observability"},
				"company":         model.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: model.NumericLabels{
				"user_id":     model.NumericLabelValue{Global: true, Value: 100},
				"cost_center": model.NumericLabelValue{Global: true, Value: 10},
			},
		},
		{
			Processor: model.LogProcessor,
			Agent: model.Agent{
				Name:    "java",
				Version: "unknown",
			},
			Service: model.Service{
				Name:     "backend",
				Language: model.Language{Name: "java"},
			},
			Message: "a random log message",
			Event: model.Event{
				Severity: int64(plog.SeverityNumberInfo),
			},
			Log:   model.Log{Level: "Info"},
			Span:  &model.Span{ID: "0200000000000000"},
			Trace: model.Trace{ID: "01000000000000000000000000000000"},
			Labels: model.Labels{
				"department_name": model.LabelValue{Global: true, Value: "apm"},
				"organization":    model.LabelValue{Global: true, Value: "observability"},
				"company":         model.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: model.NumericLabels{
				"user_id":     model.NumericLabelValue{Global: true, Value: 100},
				"cost_center": model.NumericLabelValue{Global: true, Value: 10},
			},
		},
		{
			Processor: model.ErrorProcessor,
			Agent: model.Agent{
				Name: "go",
			},
			Service: model.Service{
				Name:     "backend",
				Language: model.Language{Name: "go"},
			},
		},
		{
			Processor: model.MetricsetProcessor,
			Agent: model.Agent{
				Name: "go",
			},
			Service: model.Service{
				Name:        "backend",
				Environment: "dev",
				Language:    model.Language{Name: "go"},
			},
		},
		{
			Processor: model.SpanProcessor,
			Agent: model.Agent{
				Name: "js-base",
			},
			Service: model.Service{
				Name: "frontend",
			},
		},
	}

	var wg sync.WaitGroup
	for _, in := range apmEvents {
		wg.Add(1)
		go func(in model.APMEvent) {
			defer wg.Done()
			batch := model.Batch{in}
			err := agg.ProcessBatch(context.Background(), &batch)
			require.NoError(t, err)
			assert.Equal(t, model.Batch{in}, batch)
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
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend", Language: model.Language{Name: "java"}},
			Agent:   model.Agent{Name: "java"},
			Labels: model.Labels{
				"department_name": model.LabelValue{Value: "apm"},
				"organization":    model.LabelValue{Value: "observability"},
				"company":         model.LabelValue{Value: "elastic"},
			},
			NumericLabels: model.NumericLabels{
				"user_id":     model.NumericLabelValue{Value: 100},
				"cost_center": model.NumericLabelValue{Value: 10},
			},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend", Language: model.Language{Name: "go"}},
			Agent:   model.Agent{Name: "go"},
		}, {
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: model.Service{Name: "backend", Environment: "dev", Language: model.Language{Name: "go"}},
			Agent:   model.Agent{Name: "go"},
		}}

		assert.Equal(t, len(expected), len(metricsets))
		out := cmp.Diff(expected, metricsets, cmpopts.IgnoreTypes(netip.Addr{}), cmpopts.SortSlices(func(e1 model.APMEvent, e2 model.APMEvent) bool {
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
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       30 * time.Second,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0)
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		transaction := makeTransaction("service_name", "agent_name", "", "tx_type", "success", 100*time.Millisecond, 1)
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
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Second,
		MaxGroups:      maxGrps,
	})
	require.NoError(t, err)

	batch := make(model.Batch, maxGrps+overflowCount) // cause overflow
	for i := 0; i < len(batch); i++ {
		batch[i] = makeTransaction(
			fmt.Sprintf("svc%d", i), "java", "agent", "tx_type", "success", txnDuration, 1,
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
	monitoring.NewFunc(registry, "servicesummarymetrics", agg.CollectMonitoring)
	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["servicesummarymetrics.active_groups"] = int64(maxGrps)
	expectedMonitoring.Ints["servicesummarymetrics.overflowed.total"] = int64(overflowCount)

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
		Metricset: &model.Metricset{
			Name:     "service_summary",
			Interval: "10s",
			Samples: []model.MetricsetSample{
				{
					Name:  "service_summary.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
	}, *overflowEvent, cmpopts.IgnoreTypes(netip.Addr{}, time.Time{})))
}

func makeTransaction(
	serviceName, serviceLanguageName, agentName, transactionType, outcome string,
	duration time.Duration, count float64,
) model.APMEvent {
	return model.APMEvent{
		Agent:   model.Agent{Name: agentName},
		Service: model.Service{Name: serviceName, Language: model.Language{Name: serviceLanguageName}},
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
