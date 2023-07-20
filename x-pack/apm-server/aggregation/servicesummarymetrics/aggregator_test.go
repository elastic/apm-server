// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicesummarymetrics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	batches := make(chan modelpb.Batch, 3)
	config := AggregatorConfig{
		BatchProcessor:  makeChanBatchProcessor(batches),
		Interval:        1 * time.Millisecond,
		RollUpIntervals: []time.Duration{200 * time.Millisecond, time.Second},
		MaxGroups:       1000,
	}
	agg, err := NewAggregator(config)
	require.NoError(t, err)

	apmEvents := []*modelpb.APMEvent{
		{
			Agent:   &modelpb.Agent{Name: "java"},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "java"}},
			Event: &modelpb.Event{
				Outcome:  "success",
				Duration: durationpb.New(time.Millisecond),
			},
			Transaction: &modelpb.Transaction{
				Name:                "transaction_name",
				Type:                "request",
				RepresentativeCount: 1,
			},
			Labels: modelpb.Labels{
				"department_name": &modelpb.LabelValue{Global: true, Value: "apm"},
				"organization":    &modelpb.LabelValue{Global: true, Value: "observability"},
				"company":         &modelpb.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: modelpb.NumericLabels{
				"user_id":     &modelpb.NumericLabelValue{Global: true, Value: 100},
				"cost_center": &modelpb.NumericLabelValue{Global: true, Value: 10},
			},
		},
		{
			Agent: &modelpb.Agent{
				Name:    "java",
				Version: "unknown",
			},
			Service: &modelpb.Service{
				Name:     "backend",
				Language: &modelpb.Language{Name: "java"},
			},
			Message: "a random log message",
			Event: &modelpb.Event{
				Severity: int64(plog.SeverityNumberInfo),
			},
			Log:   &modelpb.Log{Level: "Info"},
			Span:  &modelpb.Span{Id: "0200000000000000"},
			Trace: &modelpb.Trace{Id: "01000000000000000000000000000000"},
			Labels: modelpb.Labels{
				"department_name": &modelpb.LabelValue{Global: true, Value: "apm"},
				"organization":    &modelpb.LabelValue{Global: true, Value: "observability"},
				"company":         &modelpb.LabelValue{Global: true, Value: "elastic"},
			},
			NumericLabels: modelpb.NumericLabels{
				"user_id":     &modelpb.NumericLabelValue{Global: true, Value: 100},
				"cost_center": &modelpb.NumericLabelValue{Global: true, Value: 10},
			},
		},
		{
			Agent: &modelpb.Agent{
				Name: "go",
			},
			Service: &modelpb.Service{
				Name:     "backend",
				Language: &modelpb.Language{Name: "go"},
			},
			Error: &modelpb.Error{},
		},
		{
			Agent: &modelpb.Agent{
				Name: "go",
			},
			Service: &modelpb.Service{
				Name:        "backend",
				Environment: "dev",
				Language:    &modelpb.Language{Name: "go"},
			},
		},
		{
			Agent: &modelpb.Agent{
				Name: "js-base",
			},
			Service: &modelpb.Service{
				Name: "frontend",
			},
			Span: &modelpb.Span{
				Type: "span_type",
			},
		},
	}

	var wg sync.WaitGroup
	for _, in := range apmEvents {
		wg.Add(1)
		go func(in *modelpb.APMEvent) {
			defer wg.Done()
			batch := modelpb.Batch{in}
			err := agg.ProcessBatch(context.Background(), &batch)
			require.NoError(t, err)
			assert.Equal(t, modelpb.Batch{in}, batch)
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
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "java"}},
			Agent:   &modelpb.Agent{Name: "java"},
			Labels: modelpb.Labels{
				"department_name": &modelpb.LabelValue{Value: "apm"},
				"organization":    &modelpb.LabelValue{Value: "observability"},
				"company":         &modelpb.LabelValue{Value: "elastic"},
			},
			NumericLabels: modelpb.NumericLabels{
				"user_id":     &modelpb.NumericLabelValue{Value: 100},
				"cost_center": &modelpb.NumericLabelValue{Value: 10},
			},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Language: &modelpb.Language{Name: "go"}},
			Agent:   &modelpb.Agent{Name: "go"},
		}, {
			Metricset: &modelpb.Metricset{
				Name: "service_summary", Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
			},
			Service: &modelpb.Service{Name: "backend", Environment: "dev", Language: &modelpb.Language{Name: "go"}},
			Agent:   &modelpb.Agent{Name: "go"},
		}}

		assert.Equal(t, len(expected), len(metricsets))
		assert.Empty(t, cmp.Diff(expected, metricsets,
			protocmp.Transform(),
			cmpopts.SortSlices(func(e1 *modelpb.APMEvent, e2 *modelpb.APMEvent) bool {
				if e1.Agent.Name != e2.Agent.Name {
					return e1.Agent.Name < e2.Agent.Name
				}

				return e1.Service.Environment < e2.Service.Environment

			}),
		))
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
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       30 * time.Second,
		MaxGroups:      1000,
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
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Second,
		MaxGroups:      maxGrps,
	})
	require.NoError(t, err)

	batch := make(modelpb.Batch, maxGrps+overflowCount) // cause overflow
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
		Metricset: &modelpb.Metricset{
			Name:     "service_summary",
			Interval: "10s",
			Samples: []*modelpb.MetricsetSample{
				{
					Name:  "service_summary.aggregation.overflow_count",
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
