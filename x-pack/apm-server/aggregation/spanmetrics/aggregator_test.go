// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package spanmetrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/model"
)

func BenchmarkAggregateSpan(b *testing.B) {
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeErrBatchProcessor(nil),
		Interval:       time.Minute,
		MaxGroups:      1000,
	})
	require.NoError(b, err)

	span := makeSpan("test_service", "agent", "test_destination", "success", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.ProcessBatch(context.Background(), &model.Batch{span})
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
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      1000,
	})
	require.NoError(t, err)

	type input struct {
		serviceName string
		agentName   string
		destination string
		outcome     string
		count       float64
	}

	destinationX := "destination-X"
	destinationZ := "destination-Z"
	inputs := []input{
		{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "success", count: 2},
		{serviceName: "service-A", agentName: "java", destination: destinationX, outcome: "success", count: 1},
		{serviceName: "service-B", agentName: "python", destination: destinationZ, outcome: "success", count: 1},
		{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "success", count: 1},
		{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "success", count: 0},
		{serviceName: "service-A", agentName: "java", outcome: "success", count: 1}, // no destination
		{serviceName: "service-A", agentName: "java", destination: destinationZ, outcome: "failure", count: 1},
	}

	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			span := makeSpan(in.serviceName, in.agentName, in.destination, in.outcome, 100*time.Millisecond, in.count)
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

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)

	assert.ElementsMatch(t, []model.APMEvent{{
		Agent:   model.Agent{Name: "java"},
		Service: model.Service{Name: "service-A"},
		Event:   model.Event{Outcome: "success"},
		Metricset: &model.Metricset{
			Name: "service_destination",
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: destinationX},
			},
			Samples: map[string]model.MetricsetSample{
				"span.destination.service.response_time.count":  {Value: 100.0},
				"span.destination.service.response_time.sum.us": {Value: 10000000.0},
				"metricset.period": {Value: 10},
			},
		},
	}, {
		Agent:   model.Agent{Name: "java"},
		Service: model.Service{Name: "service-A"},
		Event:   model.Event{Outcome: "failure"},
		Metricset: &model.Metricset{
			Name: "service_destination",
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: destinationZ},
			},
			Samples: map[string]model.MetricsetSample{
				"span.destination.service.response_time.count":  {Value: 100.0},
				"span.destination.service.response_time.sum.us": {Value: 10000000.0},
				"metricset.period": {Value: 10},
			},
		},
	}, {
		Agent:   model.Agent{Name: "java"},
		Service: model.Service{Name: "service-A"},
		Event:   model.Event{Outcome: "success"},
		Metricset: &model.Metricset{
			Name: "service_destination",
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: destinationZ},
			},
			Samples: map[string]model.MetricsetSample{
				"span.destination.service.response_time.count":  {Value: 300.0},
				"span.destination.service.response_time.sum.us": {Value: 30000000.0},
				"metricset.period": {Value: 10},
			},
		},
	}, {
		Agent:   model.Agent{Name: "python"},
		Service: model.Service{Name: "service-B"},
		Event:   model.Event{Outcome: "success"},
		Metricset: &model.Metricset{
			Name: "service_destination",
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: destinationZ},
			},
			Samples: map[string]model.MetricsetSample{
				"span.destination.service.response_time.count":  {Value: 100.0},
				"span.destination.service.response_time.sum.us": {Value: 10000000.0},
				"metricset.period": {Value: 10},
			},
		},
	}}, metricsets)

	select {
	case <-batches:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregatorOverflow(t *testing.T) {
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      2,
	})
	require.NoError(t, err)

	// The first two transaction groups will not require immediate publication,
	// as we have configured the spanmetrics with a maximum of two buckets.
	batch := make(model.Batch, 20)
	for i := 0; i < len(batch); i += 2 {
		batch[i] = makeSpan("service", "agent", "destination1", "success", 100*time.Millisecond, 1)
		batch[i+1] = makeSpan("service", "agent", "destination2", "success", 100*time.Millisecond, 1)
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))

	// The third group will return a metricset for immediate publication.
	for i := 0; i < 2; i++ {
		batch = append(batch, makeSpan("service", "agent", "destination3", "success", 100*time.Millisecond, 1))
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metricsets := batchMetricsets(t, batch)
	assert.Len(t, metricsets, 2)

	for _, m := range metricsets {
		assert.Equal(t, model.APMEvent{
			Agent:   model.Agent{Name: "agent"},
			Service: model.Service{Name: "service"},
			Event:   model.Event{Outcome: "success"},
			Metricset: &model.Metricset{
				Name: "service_destination",
				Span: model.MetricsetSpan{
					DestinationService: model.DestinationService{Resource: "destination3"},
				},
				Samples: map[string]model.MetricsetSample{
					"span.destination.service.response_time.count":  {Value: 1.0},
					"span.destination.service.response_time.sum.us": {Value: 100000.0},
					// No metricset.period is recorded as these metrics are instantanous, not aggregated.
				},
			},
		}, m)
	}
}

func makeSpan(
	serviceName, agentName, destinationServiceResource, outcome string,
	duration time.Duration,
	count float64,
) model.APMEvent {
	event := model.APMEvent{
		Agent:   model.Agent{Name: agentName},
		Service: model.Service{Name: serviceName},
		Event:   model.Event{Outcome: outcome},
		Span: &model.Span{
			Name:                serviceName + ":" + destinationServiceResource,
			Duration:            duration.Seconds() * 1000,
			RepresentativeCount: count,
		},
	}
	if destinationServiceResource != "" {
		event.Span.DestinationService = &model.DestinationService{
			Resource: destinationServiceResource,
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
		require.NotZero(t, event.Timestamp)
		event.Timestamp = time.Time{}
		metricsets = append(metricsets, event)
	}
	return metricsets
}
