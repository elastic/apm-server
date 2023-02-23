// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package spanmetrics

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
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func BenchmarkAggregateSpan(b *testing.B) {
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeErrBatchProcessor(nil),
		Interval:       time.Minute,
		MaxGroups:      1000,
	})
	require.NoError(b, err)

	span := makeSpan("test_service", "agent", "test_destination", "trg_type", "trg_name", "success", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = agg.ProcessBatch(context.Background(), &model.Batch{span})
		}
	})
}

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
		Interval:        10 * time.Millisecond,
		RollUpIntervals: []time.Duration{200 * time.Millisecond, time.Second},
		MaxGroups:       1000,
	}
	agg, err := NewAggregator(config)
	require.NoError(t, err)

	type input struct {
		serviceName string
		agentName   string
		destination string
		targetType  string
		targetName  string
		outcome     string
		count       float64
	}

	destinationX := "destination-X"
	destinationZ := "destination-Z"
	trgTypeX := "trg-type-X"
	trgNameX := "trg-name-X"
	trgTypeZ := "trg-type-Z"
	trgNameZ := "trg-name-Z"
	inputs := []input{
		{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 2},
		{serviceName: "service-A", agentName: "java", destination: destinationX, targetType: trgTypeX, targetName: trgNameX, outcome: "success", count: 1},
		{serviceName: "service-B", agentName: "python", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1},
		{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1},
		{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 0},
		{serviceName: "service-A", agentName: "java", outcome: "success", count: 1},                                             // no destination or service target
		{serviceName: "service-A", agentName: "java", targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1}, // no destination
		{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "success", count: 1},                  // no service target
		{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "failure", count: 1},
	}

	var wg sync.WaitGroup
	now := time.Now()
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			span := makeSpan(in.serviceName, in.agentName, in.destination, in.targetType, in.targetName, in.outcome, 100*time.Millisecond, in.count)
			span.Timestamp = now
			batch := model.Batch{span}
			for i := 0; i < 100; i++ {
				err := agg.ProcessBatch(context.Background(), &batch)
				require.NoError(t, err)
				assert.Equal(t, model.Batch{span}, batch)
			}
		}(in)
	}
	wg.Wait()

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())
	// Stop the aggregator to ensure all metrics are published.
	assert.NoError(t, agg.Stop(context.Background()))

	for _, interval := range append([]time.Duration{config.Interval}, config.RollUpIntervals...) {
		metricsets := batchMetricsets(t, expectBatch(t, batches))

		assert.ElementsMatch(t, []model.APMEvent{{
			Timestamp: now.Truncate(interval),
			Agent:     model.Agent{Name: "java"},
			Service: model.Service{
				Name: "service-A",
				Target: &model.ServiceTarget{
					Type: trgTypeX,
					Name: trgNameX,
				},
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:     "service_destination",
				Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
				DocCount: 100,
			},
			Span: &model.Span{
				Name: "service-A:" + destinationX,
				DestinationService: &model.DestinationService{
					Resource: destinationX,
					ResponseTime: model.AggregatedDuration{
						Count: 100,
						Sum:   10 * time.Second,
					},
				},
			},
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
			Timestamp: now.Truncate(interval),
			Agent:     model.Agent{Name: "java"},
			Service: model.Service{
				Name: "service-A",
				Target: &model.ServiceTarget{
					Type: trgTypeZ,
					Name: trgNameZ,
				},
			},
			Event:     model.Event{Outcome: "failure"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:     "service_destination",
				Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
				DocCount: 100,
			},
			Span: &model.Span{
				Name: "service-A:" + destinationZ,
				DestinationService: &model.DestinationService{
					Resource: destinationZ,
					ResponseTime: model.AggregatedDuration{
						Count: 100,
						Sum:   10 * time.Second,
					},
				},
			},
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
			Timestamp: now.Truncate(interval),
			Agent:     model.Agent{Name: "java"},
			Service: model.Service{
				Name: "service-A",
				Target: &model.ServiceTarget{
					Type: trgTypeZ,
					Name: trgNameZ,
				},
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:     "service_destination",
				Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
				DocCount: 300,
			},
			Span: &model.Span{
				Name: "service-A:" + destinationZ,
				DestinationService: &model.DestinationService{
					Resource: destinationZ,
					ResponseTime: model.AggregatedDuration{
						Count: 300,
						Sum:   30 * time.Second,
					},
				},
			},
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
			Timestamp: now.Truncate(interval),
			Agent:     model.Agent{Name: "java"},
			Service: model.Service{
				Name: "service-A",
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:     "service_destination",
				Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
				DocCount: 100,
			},
			Span: &model.Span{
				Name: "service-A:" + destinationZ,
				DestinationService: &model.DestinationService{
					Resource: destinationZ,
					ResponseTime: model.AggregatedDuration{
						Count: 100,
						Sum:   10 * time.Second,
					},
				},
			},
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
			Timestamp: now.Truncate(interval),
			Agent:     model.Agent{Name: "python"},
			Service: model.Service{
				Name: "service-B",
				Target: &model.ServiceTarget{
					Type: trgTypeZ,
					Name: trgNameZ,
				},
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{
				Name:     "service_destination",
				Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
				DocCount: 100,
			},
			Span: &model.Span{
				Name: "service-B:" + destinationZ,
				DestinationService: &model.DestinationService{
					Resource: destinationZ,
					ResponseTime: model.AggregatedDuration{
						Count: 100,
						Sum:   10 * time.Second,
					},
				},
			},
			Labels: model.Labels{
				"department_name": model.LabelValue{Value: "apm"},
				"organization":    model.LabelValue{Value: "observability"},
				"company":         model.LabelValue{Value: "elastic"},
			},
			NumericLabels: model.NumericLabels{
				"user_id":     model.NumericLabelValue{Value: 100},
				"cost_center": model.NumericLabelValue{Value: 10},
			},
		}}, metricsets)
	}

	select {
	case <-batches:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregateCompositeSpan(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	span := makeSpan("service-A", "java", "final_destination", "trg_type", "trg_name", "success", time.Second, 2)
	span.Span.Composite = &model.Composite{Count: 25, Sum: 700 /* milliseconds */}
	err = agg.ProcessBatch(context.Background(), &model.Batch{span})
	require.NoError(t, err)

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	assert.Equal(t, []model.APMEvent{{
		Agent: model.Agent{Name: "java"},
		Service: model.Service{
			Name: "service-A",
			Target: &model.ServiceTarget{
				Type: "trg_type",
				Name: "trg_name",
			},
		},
		Event:     model.Event{Outcome: "success"},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service_destination", Interval: "0s", DocCount: 50},
		Span: &model.Span{
			Name: "service-A:final_destination",
			DestinationService: &model.DestinationService{
				Resource: "final_destination",
				ResponseTime: model.AggregatedDuration{
					Count: 50,
					Sum:   1400 * time.Millisecond,
				},
			},
		},
		Labels: model.Labels{
			"department_name": model.LabelValue{Value: "apm"},
			"organization":    model.LabelValue{Value: "observability"},
			"company":         model.LabelValue{Value: "elastic"},
		},
		NumericLabels: model.NumericLabels{
			"user_id":     model.NumericLabelValue{Value: 100},
			"cost_center": model.NumericLabelValue{Value: 10},
		},
	}}, metricsets)
}

func TestAggregateTransactionDroppedSpansStats(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	txWithoutServiceTarget := model.APMEvent{
		Agent: model.Agent{Name: "go"},
		Service: model.Service{
			Name: "go-service",
		},
		Event: model.Event{
			Outcome:  "success",
			Duration: 10 * time.Second,
		},
		Processor: model.TransactionProcessor,
		Transaction: &model.Transaction{
			RepresentativeCount: 2,
			DroppedSpansStats: []model.DroppedSpanStats{
				{
					DestinationServiceResource: "https://elasticsearch:9200",
					Outcome:                    "success",
					Duration: model.AggregatedDuration{
						Count: 10,
						Sum:   1500 * time.Microsecond,
					},
				},
				{
					DestinationServiceResource: "mysql://mysql:3306",
					Outcome:                    "unknown",
					Duration: model.AggregatedDuration{
						Count: 2,
						Sum:   3000 * time.Microsecond,
					},
				},
			},
		},
	}

	txWithServiceTarget := model.APMEvent{
		Agent: model.Agent{Name: "go"},
		Service: model.Service{
			Name: "go-service",
		},
		Event: model.Event{
			Outcome:  "success",
			Duration: 10 * time.Second,
		},
		Processor: model.TransactionProcessor,
		Transaction: &model.Transaction{
			RepresentativeCount: 1,
			DroppedSpansStats: []model.DroppedSpanStats{
				{
					DestinationServiceResource: "postgres/testdb",
					ServiceTargetType:          "postgres",
					ServiceTargetName:          "testdb",
					Outcome:                    "success",
					Duration: model.AggregatedDuration{
						Count: 10,
						Sum:   1500 * time.Microsecond,
					},
				},
			},
		},
	}

	txWithNoRepresentativeCount := model.APMEvent{
		Processor: model.TransactionProcessor,
		Transaction: &model.Transaction{
			RepresentativeCount: 0,
			DroppedSpansStats:   make([]model.DroppedSpanStats, 1),
		},
	}
	err = agg.ProcessBatch(
		context.Background(),
		&model.Batch{
			txWithoutServiceTarget,
			txWithServiceTarget,
			txWithNoRepresentativeCount,
		})
	require.NoError(t, err)

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	assert.ElementsMatch(t, []model.APMEvent{
		{
			Agent: model.Agent{Name: "go"},
			Service: model.Service{
				Name: "go-service",
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{Name: "service_destination", Interval: "0s", DocCount: 20},
			Span: &model.Span{
				DestinationService: &model.DestinationService{
					Resource: "https://elasticsearch:9200",
					ResponseTime: model.AggregatedDuration{
						Count: 20,
						Sum:   3000 * time.Microsecond,
					},
				},
			},
		},
		{
			Agent: model.Agent{Name: "go"},
			Service: model.Service{
				Name: "go-service",
				Target: &model.ServiceTarget{
					Type: "postgres",
					Name: "testdb",
				},
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{Name: "service_destination", Interval: "0s", DocCount: 10},
			Span: &model.Span{
				DestinationService: &model.DestinationService{
					Resource: "postgres/testdb",
					ResponseTime: model.AggregatedDuration{
						Count: 10,
						Sum:   1500 * time.Microsecond,
					},
				},
			},
		},
		{
			Agent: model.Agent{Name: "go"},
			Service: model.Service{
				Name: "go-service",
			},
			Event:     model.Event{Outcome: "success"},
			Processor: model.MetricsetProcessor,
			Metricset: &model.Metricset{Name: "service_destination", Interval: "0s", DocCount: 4},
			Span: &model.Span{
				DestinationService: &model.DestinationService{
					Resource: "mysql://mysql:3306",
					ResponseTime: model.AggregatedDuration{
						Count: 4,
						Sum:   6000 * time.Microsecond,
					},
				},
			},
		},
	}, metricsets)
}

func TestAggregateHalfCapacityNoSpanName(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      4,
	})
	require.NoError(t, err)

	err = agg.ProcessBatch(
		context.Background(),
		&model.Batch{
			makeSpan("service", "agent", "dest1", "target_type", "target1", "success", 100*time.Millisecond, 1),
			makeSpan("service", "agent", "dest2", "target_type", "target2", "success", 100*time.Millisecond, 1),
			makeSpan("service", "agent", "dest3", "target_type", "target3", "success", 100*time.Millisecond, 1),
			makeSpan("service", "agent", "dest4", "target_type", "target4", "success", 100*time.Millisecond, 1),
		},
	)
	require.NoError(t, err)

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	actualDestinationSpanNames := make(map[string]string)
	for _, ms := range metricsets {
		actualDestinationSpanNames[ms.Span.DestinationService.Resource] = ms.Span.Name
	}
	assert.Equal(t, map[string]string{
		"dest1": "service:dest1",
		"dest2": "service:dest2",
		// After 50% capacity (4 buckets with our configuration) is reached,
		// the remaining metrics are aggregated without span.name.
		"dest3": "",
		"dest4": "",
	}, actualDestinationSpanNames)
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
		span := makeSpan("service_name", "agent_name", "destination", "trg_type", "trg_name", "success", 100*time.Millisecond, 1)
		span.Timestamp = ts
		batch := model.Batch{span}
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
	duration := 100 * time.Millisecond
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Second,
		MaxGroups:      maxGrps,
	})
	require.NoError(t, err)

	batch := make(model.Batch, maxGrps+overflowCount) // cause overflow
	for i := 0; i < len(batch); i++ {
		batch[i] = makeSpan("service", "agent", fmt.Sprintf("destination%d", i),
			fmt.Sprintf("trg_type_%d", i), fmt.Sprintf("trg_name_%d", i), "success", duration, 1)
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
	monitoring.NewFunc(registry, "spanmetrics", agg.CollectMonitoring)
	expectedMonitoring := monitoring.MakeFlatSnapshot()
	expectedMonitoring.Ints["spanmetrics.active_groups"] = int64(maxGrps)
	expectedMonitoring.Ints["spanmetrics.overflowed.total"] = int64(overflowCount)

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
			Name:     "service_destination",
			DocCount: int64(overflowCount),
			Interval: "10s",
			Samples: []model.MetricsetSample{
				{
					Name:  "service_destination.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
		Span: &model.Span{
			Name: "",
			DestinationService: &model.DestinationService{
				Resource: "",
				ResponseTime: model.AggregatedDuration{
					Count: overflowCount,
					Sum:   time.Duration(duration.Nanoseconds() * int64(overflowCount)),
				},
			},
		},
	}, *overflowEvent, cmpopts.IgnoreTypes(netip.Addr{}, time.Time{})))
}

func makeSpan(
	serviceName, agentName, destinationServiceResource, targetType, targetName, outcome string,
	duration time.Duration,
	count float64,
) model.APMEvent {
	event := model.APMEvent{
		Agent:   model.Agent{Name: agentName},
		Service: model.Service{Name: serviceName},
		Event: model.Event{
			Outcome:  outcome,
			Duration: duration,
		},
		Processor: model.SpanProcessor,
		Span: &model.Span{
			Name:                serviceName + ":" + destinationServiceResource,
			RepresentativeCount: count,
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
	}
	if destinationServiceResource != "" {
		event.Span.DestinationService = &model.DestinationService{
			Resource: destinationServiceResource,
		}
	}
	if targetType != "" {
		event.Service.Target = &model.ServiceTarget{
			Type: targetType,
			Name: targetName,
		}
	}
	return event
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
