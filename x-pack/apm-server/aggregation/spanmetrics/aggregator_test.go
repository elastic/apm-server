// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package spanmetrics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/apm-data/model/modelpb"
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
			_ = agg.ProcessBatch(context.Background(), &modelpb.Batch{span})
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

	for _, tt := range []struct {
		name string

		inputs []input

		getExpectedEvents func(time.Time, time.Duration) []*modelpb.APMEvent
	}{
		{
			name: "with destination and service targets",

			inputs: []input{
				{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 2},
				{serviceName: "service-A", agentName: "java", destination: destinationX, targetType: trgTypeX, targetName: trgNameX, outcome: "success", count: 1},
				{serviceName: "service-B", agentName: "python", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1},
				{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1},
				{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 0},
				{serviceName: "service-A", agentName: "java", destination: destinationZ, targetType: trgTypeZ, targetName: trgNameZ, outcome: "failure", count: 1},
			},

			getExpectedEvents: func(now time.Time, interval time.Duration) []*modelpb.APMEvent {
				return []*modelpb.APMEvent{
					{
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "java"},
						Service: &modelpb.Service{
							Name: "service-A",
							Target: &modelpb.ServiceTarget{
								Type: trgTypeX,
								Name: trgNameX,
							},
						},
						Event: &modelpb.Event{Outcome: "success"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 100,
						},
						Span: &modelpb.Span{
							Name: "service-A:" + destinationX,
							DestinationService: &modelpb.DestinationService{
								Resource: destinationX,
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 100,
									Sum:   durationpb.New(10 * time.Second),
								},
							},
						},
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
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "java"},
						Service: &modelpb.Service{
							Name: "service-A",
							Target: &modelpb.ServiceTarget{
								Type: trgTypeZ,
								Name: trgNameZ,
							},
						},
						Event: &modelpb.Event{Outcome: "failure"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 100,
						},
						Span: &modelpb.Span{
							Name: "service-A:" + destinationZ,
							DestinationService: &modelpb.DestinationService{
								Resource: destinationZ,
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 100,
									Sum:   durationpb.New(10 * time.Second),
								},
							},
						},
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
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "java"},
						Service: &modelpb.Service{
							Name: "service-A",
							Target: &modelpb.ServiceTarget{
								Type: trgTypeZ,
								Name: trgNameZ,
							},
						},
						Event: &modelpb.Event{Outcome: "success"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 300,
						},
						Span: &modelpb.Span{
							Name: "service-A:" + destinationZ,
							DestinationService: &modelpb.DestinationService{
								Resource: destinationZ,
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 300,
									Sum:   durationpb.New(30 * time.Second),
								},
							},
						},
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
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "python"},
						Service: &modelpb.Service{
							Name: "service-B",
							Target: &modelpb.ServiceTarget{
								Type: trgTypeZ,
								Name: trgNameZ,
							},
						},
						Event: &modelpb.Event{Outcome: "success"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 100,
						},
						Span: &modelpb.Span{
							Name: "service-B:" + destinationZ,
							DestinationService: &modelpb.DestinationService{
								Resource: destinationZ,
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 100,
									Sum:   durationpb.New(10 * time.Second),
								},
							},
						},
						Labels: modelpb.Labels{
							"department_name": &modelpb.LabelValue{Value: "apm"},
							"organization":    &modelpb.LabelValue{Value: "observability"},
							"company":         &modelpb.LabelValue{Value: "elastic"},
						},
						NumericLabels: modelpb.NumericLabels{
							"user_id":     &modelpb.NumericLabelValue{Value: 100},
							"cost_center": &modelpb.NumericLabelValue{Value: 10},
						},
					},
				}
			},
		},
		{
			name: "with no destination and no service target",

			inputs: []input{
				{serviceName: "service-A", agentName: "java", outcome: "success", count: 1},
			},
			getExpectedEvents: func(now time.Time, interval time.Duration) []*modelpb.APMEvent {
				return []*modelpb.APMEvent{}
			},
		},
		{
			name: "with no destination and a service target",

			inputs: []input{
				{serviceName: "service-A", agentName: "java", targetType: trgTypeZ, targetName: trgNameZ, outcome: "success", count: 1},
			},
			getExpectedEvents: func(now time.Time, interval time.Duration) []*modelpb.APMEvent {
				return []*modelpb.APMEvent{
					{
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "java"},
						Service: &modelpb.Service{
							Name: "service-A",
							Target: &modelpb.ServiceTarget{
								Type: trgTypeZ,
								Name: trgNameZ,
							},
						},
						Event: &modelpb.Event{Outcome: "success"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 100,
						},
						Span: &modelpb.Span{
							Name: "service-A:",
							DestinationService: &modelpb.DestinationService{
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 100,
									Sum:   durationpb.New(10 * time.Second),
								},
							},
						},
						Labels: modelpb.Labels{
							"department_name": &modelpb.LabelValue{Value: "apm"},
							"organization":    &modelpb.LabelValue{Value: "observability"},
							"company":         &modelpb.LabelValue{Value: "elastic"},
						},
						NumericLabels: modelpb.NumericLabels{
							"user_id":     &modelpb.NumericLabelValue{Value: 100},
							"cost_center": &modelpb.NumericLabelValue{Value: 10},
						},
					},
				}
			},
		},
		{
			name: "with a destination and no service target",

			inputs: []input{
				{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "success", count: 1},
			},
			getExpectedEvents: func(now time.Time, interval time.Duration) []*modelpb.APMEvent {
				return []*modelpb.APMEvent{
					{
						Timestamp: timestamppb.New(now.Truncate(interval)),
						Agent:     &modelpb.Agent{Name: "java"},
						Service: &modelpb.Service{
							Name: "service-A",
						},
						Event: &modelpb.Event{Outcome: "success"},
						Metricset: &modelpb.Metricset{
							Name:     "service_destination",
							Interval: fmt.Sprintf("%.0fs", interval.Seconds()),
							DocCount: 100,
						},
						Span: &modelpb.Span{
							Name: "service-A:" + destinationZ,
							DestinationService: &modelpb.DestinationService{
								Resource: destinationZ,
								ResponseTime: &modelpb.AggregatedDuration{
									Count: 100,
									Sum:   durationpb.New(10 * time.Second),
								},
							},
						},
						Labels: modelpb.Labels{
							"department_name": &modelpb.LabelValue{Value: "apm"},
							"organization":    &modelpb.LabelValue{Value: "observability"},
							"company":         &modelpb.LabelValue{Value: "elastic"},
						},
						NumericLabels: modelpb.NumericLabels{
							"user_id":     &modelpb.NumericLabelValue{Value: 100},
							"cost_center": &modelpb.NumericLabelValue{Value: 10},
						},
					},
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			batches := make(chan modelpb.Batch, 3)
			config := AggregatorConfig{
				BatchProcessor:  makeChanBatchProcessor(batches),
				Interval:        10 * time.Millisecond,
				RollUpIntervals: []time.Duration{200 * time.Millisecond, time.Second},
				MaxGroups:       1000,
			}
			agg, err := NewAggregator(config)
			require.NoError(t, err)

			var wg sync.WaitGroup
			now := time.Now()

			for _, in := range tt.inputs {
				wg.Add(1)
				go func(in input) {
					defer wg.Done()
					span := makeSpan(in.serviceName, in.agentName, in.destination, in.targetType, in.targetName, in.outcome, 100*time.Millisecond, in.count)
					span.Timestamp = timestamppb.New(now)
					batch := modelpb.Batch{span}
					for i := 0; i < 100; i++ {
						err := agg.ProcessBatch(context.Background(), &batch)
						require.NoError(t, err)
						assert.Equal(t, modelpb.Batch{span}, batch)
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
				expectedEvents := tt.getExpectedEvents(now, interval)

				if len(expectedEvents) == 0 {
					select {
					case <-batches:
						assert.Fail(t, "unexpected publish")
					case <-time.After(100 * time.Millisecond):
					}
				} else {
					metricsets := batchMetricsets(t, expectBatch(t, batches))
					assert.Empty(t, cmp.Diff(expectedEvents, metricsets,
						protocmp.Transform(),
						cmpopts.SortSlices(func(x, y *modelpb.APMEvent) bool {
							if x.Span.Name != y.Span.Name {
								return x.Span.Name < y.Span.Name
							}
							if x.Event.Outcome != y.Event.Outcome {
								return x.Event.Outcome < y.Event.Outcome
							}
							return x.Agent.Name < y.Agent.Name
						}),
					))
				}
			}

			select {
			case <-batches:
				t.Fatal("unexpected publish")
			case <-time.After(100 * time.Millisecond):
			}
		})
	}

}

func TestAggregateCompositeSpan(t *testing.T) {
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	span := makeSpan("service-A", "java", "final_destination", "trg_type", "trg_name", "success", time.Second, 2)
	span.Span.Composite = &modelpb.Composite{Count: 25, Sum: 700 /* milliseconds */}
	err = agg.ProcessBatch(context.Background(), &modelpb.Batch{span})
	require.NoError(t, err)

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	assert.Empty(t, cmp.Diff([]*modelpb.APMEvent{{
		Agent: &modelpb.Agent{Name: "java"},
		Service: &modelpb.Service{
			Name: "service-A",
			Target: &modelpb.ServiceTarget{
				Type: "trg_type",
				Name: "trg_name",
			},
		},
		Event:     &modelpb.Event{Outcome: "success"},
		Metricset: &modelpb.Metricset{Name: "service_destination", Interval: "0s", DocCount: 50},
		Span: &modelpb.Span{
			Name: "service-A:final_destination",
			DestinationService: &modelpb.DestinationService{
				Resource: "final_destination",
				ResponseTime: &modelpb.AggregatedDuration{
					Count: 50,
					Sum:   durationpb.New(1400 * time.Millisecond),
				},
			},
		},
		Labels: modelpb.Labels{
			"department_name": &modelpb.LabelValue{Value: "apm"},
			"organization":    &modelpb.LabelValue{Value: "observability"},
			"company":         &modelpb.LabelValue{Value: "elastic"},
		},
		NumericLabels: modelpb.NumericLabels{
			"user_id":     &modelpb.NumericLabelValue{Value: 100},
			"cost_center": &modelpb.NumericLabelValue{Value: 10},
		},
	}}, metricsets, protocmp.Transform()))
}

func TestAggregateTransactionDroppedSpansStats(t *testing.T) {
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	txWithoutServiceTarget := modelpb.APMEvent{
		Agent: &modelpb.Agent{Name: "go"},
		Service: &modelpb.Service{
			Name: "go-service",
		},
		Event: &modelpb.Event{
			Outcome:  "success",
			Duration: durationpb.New(10 * time.Second),
		},
		Transaction: &modelpb.Transaction{
			Type:                "transaction_type",
			RepresentativeCount: 2,
			DroppedSpansStats: []*modelpb.DroppedSpanStats{
				{
					DestinationServiceResource: "https://elasticsearch:9200",
					Outcome:                    "success",
					Duration: &modelpb.AggregatedDuration{
						Count: 10,
						Sum:   durationpb.New(1500 * time.Microsecond),
					},
				},
				{
					DestinationServiceResource: "mysql://mysql:3306",
					Outcome:                    "unknown",
					Duration: &modelpb.AggregatedDuration{
						Count: 2,
						Sum:   durationpb.New(3000 * time.Microsecond),
					},
				},
			},
		},
	}

	txWithServiceTarget := modelpb.APMEvent{
		Agent: &modelpb.Agent{Name: "go"},
		Service: &modelpb.Service{
			Name: "go-service",
		},
		Event: &modelpb.Event{
			Outcome:  "success",
			Duration: durationpb.New(10 * time.Second),
		},
		Transaction: &modelpb.Transaction{
			Type:                "transaction_type",
			RepresentativeCount: 1,
			DroppedSpansStats: []*modelpb.DroppedSpanStats{
				{
					DestinationServiceResource: "postgres/testdb",
					ServiceTargetType:          "postgres",
					ServiceTargetName:          "testdb",
					Outcome:                    "success",
					Duration: &modelpb.AggregatedDuration{
						Count: 10,
						Sum:   durationpb.New(1500 * time.Microsecond),
					},
				},
			},
		},
	}

	txWithNoRepresentativeCount := modelpb.APMEvent{
		Transaction: &modelpb.Transaction{
			Type:                "transaction_type",
			RepresentativeCount: 0,
			DroppedSpansStats:   make([]*modelpb.DroppedSpanStats, 1),
		},
	}
	err = agg.ProcessBatch(
		context.Background(),
		&modelpb.Batch{
			&txWithoutServiceTarget,
			&txWithServiceTarget,
			&txWithNoRepresentativeCount,
		})
	require.NoError(t, err)

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	assert.Empty(t, cmp.Diff([]*modelpb.APMEvent{
		{
			Agent: &modelpb.Agent{Name: "go"},
			Service: &modelpb.Service{
				Name: "go-service",
			},
			Event:     &modelpb.Event{Outcome: "success"},
			Metricset: &modelpb.Metricset{Name: "service_destination", Interval: "0s", DocCount: 20},
			Span: &modelpb.Span{
				DestinationService: &modelpb.DestinationService{
					Resource: "https://elasticsearch:9200",
					ResponseTime: &modelpb.AggregatedDuration{
						Count: 20,
						Sum:   durationpb.New(3000 * time.Microsecond),
					},
				},
			},
		},
		{
			Agent: &modelpb.Agent{Name: "go"},
			Service: &modelpb.Service{
				Name: "go-service",
				Target: &modelpb.ServiceTarget{
					Type: "postgres",
					Name: "testdb",
				},
			},
			Event:     &modelpb.Event{Outcome: "success"},
			Metricset: &modelpb.Metricset{Name: "service_destination", Interval: "0s", DocCount: 10},
			Span: &modelpb.Span{
				DestinationService: &modelpb.DestinationService{
					Resource: "postgres/testdb",
					ResponseTime: &modelpb.AggregatedDuration{
						Count: 10,
						Sum:   durationpb.New(1500 * time.Microsecond),
					},
				},
			},
		},
		{
			Agent: &modelpb.Agent{Name: "go"},
			Service: &modelpb.Service{
				Name: "go-service",
			},
			Event:     &modelpb.Event{Outcome: "unknown"},
			Metricset: &modelpb.Metricset{Name: "service_destination", Interval: "0s", DocCount: 4},
			Span: &modelpb.Span{
				DestinationService: &modelpb.DestinationService{
					Resource: "mysql://mysql:3306",
					ResponseTime: &modelpb.AggregatedDuration{
						Count: 4,
						Sum:   durationpb.New(6000 * time.Microsecond),
					},
				},
			},
		},
	}, metricsets,
		protocmp.Transform(),
		cmpopts.SortSlices(func(e1 *modelpb.APMEvent, e2 *modelpb.APMEvent) bool {
			return e1.Span.DestinationService.Resource < e2.Span.DestinationService.Resource
		})))
}

func TestAggregateHalfCapacityNoSpanName(t *testing.T) {
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      4,
	})
	require.NoError(t, err)

	err = agg.ProcessBatch(
		context.Background(),
		&modelpb.Batch{
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
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       30 * time.Second,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	t0 := time.Unix(0, 0).UTC()
	for _, ts := range []time.Time{t0, t0.Add(15 * time.Second), t0.Add(30 * time.Second)} {
		span := makeSpan("service_name", "agent_name", "destination", "trg_type", "trg_name", "success", 100*time.Millisecond, 1)
		span.Timestamp = timestamppb.New(ts)
		batch := modelpb.Batch{span}
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
	duration := 100 * time.Millisecond
	batches := make(chan modelpb.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Second,
		MaxGroups:      maxGrps,
	})
	require.NoError(t, err)

	batch := make(modelpb.Batch, maxGrps+overflowCount) // cause overflow
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
			Name:     "service_destination",
			DocCount: int64(overflowCount),
			Interval: "10s",
			Samples: []*modelpb.MetricsetSample{
				{
					Name:  "service_destination.aggregation.overflow_count",
					Value: float64(overflowCount),
				},
			},
		},
		Span: &modelpb.Span{
			Name: "",
			DestinationService: &modelpb.DestinationService{
				Resource: "",
				ResponseTime: &modelpb.AggregatedDuration{
					Count: int64(overflowCount),
					Sum:   durationpb.New(time.Duration(duration.Nanoseconds() * int64(overflowCount))),
				},
			},
		},
	}, overflowEvent,
		protocmp.Transform(),
		protocmp.IgnoreMessages(&timestamppb.Timestamp{}),
	))
}

func makeSpan(
	serviceName, agentName, destinationServiceResource, targetType, targetName, outcome string,
	duration time.Duration,
	count float64,
) *modelpb.APMEvent {
	event := modelpb.APMEvent{
		Agent:   &modelpb.Agent{Name: agentName},
		Service: &modelpb.Service{Name: serviceName},
		Event: &modelpb.Event{
			Outcome:  outcome,
			Duration: durationpb.New(duration),
		},
		Span: &modelpb.Span{
			Type:                "span_type",
			Name:                serviceName + ":" + destinationServiceResource,
			RepresentativeCount: count,
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
	}
	if destinationServiceResource != "" {
		event.Span.DestinationService = &modelpb.DestinationService{
			Resource: destinationServiceResource,
		}
	}
	if targetType != "" {
		event.Service.Target = &modelpb.ServiceTarget{
			Type: targetType,
			Name: targetName,
		}
	}
	return &event
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
