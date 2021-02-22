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
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func BenchmarkAggregateSpan(b *testing.B) {
	agg, err := NewAggregator(AggregatorConfig{
		Report:    makeErrReporter(nil),
		Interval:  time.Minute,
		MaxGroups: 1000,
	})
	require.NoError(b, err)

	span := makeSpan("test_service", "agent", "test_destination", "success", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.ProcessTransformables(
				context.Background(),
				[]transform.Transformable{span},
			)
		}
	})
}

func TestNewAggregatorConfigInvalid(t *testing.T) {
	report := makeErrReporter(nil)

	type test struct {
		config AggregatorConfig
		err    string
	}

	for _, test := range []test{{
		config: AggregatorConfig{},
		err:    "Report unspecified",
	}, {
		config: AggregatorConfig{
			Report: report,
		},
		err: "MaxGroups unspecified or negative",
	}, {
		config: AggregatorConfig{
			Report:    report,
			MaxGroups: 1,
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
	reqs := make(chan publish.PendingReq, 1)
	agg, err := NewAggregator(AggregatorConfig{
		Report:    makeChanReporter(reqs),
		Interval:  10 * time.Millisecond,
		MaxGroups: 1000,
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
			transformables := []transform.Transformable{span}
			for i := 0; i < 100; i++ {
				out, err := agg.ProcessTransformables(context.Background(), transformables)
				require.NoError(t, err)
				assert.Equal(t, transformables, out)
			}
		}(in)
	}
	wg.Wait()

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	req := expectPublish(t, reqs)
	metricsets := make([]*model.Metricset, len(req.Transformables))
	for i, tf := range req.Transformables {
		ms := tf.(*model.Metricset)
		require.NotZero(t, ms.Timestamp)
		ms.Timestamp = time.Time{}
		metricsets[i] = ms
	}

	assert.ElementsMatch(t, []*model.Metricset{{
		Name: "service_destination",
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A", Agent: model.Agent{Name: "java"}},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationX},
		},
		Samples: []model.Sample{
			{Name: "span.destination.service.response_time.count", Value: 100.0},
			{Name: "span.destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Name: "service_destination",
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A", Agent: model.Agent{Name: "java"}},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "failure",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "span.destination.service.response_time.count", Value: 100.0},
			{Name: "span.destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Name: "service_destination",
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A", Agent: model.Agent{Name: "java"}},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "span.destination.service.response_time.count", Value: 300.0},
			{Name: "span.destination.service.response_time.sum.us", Value: 30000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Name: "service_destination",
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-B", Agent: model.Agent{Name: "python"}},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "span.destination.service.response_time.count", Value: 100.0},
			{Name: "span.destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}}, metricsets)

	select {
	case <-reqs:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregatorOverflow(t *testing.T) {
	reqs := make(chan publish.PendingReq, 1)
	agg, err := NewAggregator(AggregatorConfig{
		Report:    makeChanReporter(reqs),
		Interval:  10 * time.Millisecond,
		MaxGroups: 2,
	})
	require.NoError(t, err)

	// The first two transaction groups will not require immediate publication,
	// as we have configured the spanmetrics with a maximum of two buckets.
	var input []transform.Transformable
	for i := 0; i < 10; i++ {
		input = append(input, makeSpan("service", "agent", "destination1", "success", 100*time.Millisecond, 1))
		input = append(input, makeSpan("service", "agent", "destination2", "success", 100*time.Millisecond, 1))
	}
	output, err := agg.ProcessTransformables(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, input, output)

	// The third group will return a metricset for immediate publication.
	for i := 0; i < 2; i++ {
		input = append(input, makeSpan("service", "agent", "destination3", "success", 100*time.Millisecond, 1))
	}
	output, err = agg.ProcessTransformables(context.Background(), input)
	require.NoError(t, err)
	assert.Len(t, output, len(input)+2)
	assert.Equal(t, input, output[:len(input)])

	for _, tf := range output[len(input):] {
		m, ok := tf.(*model.Metricset)
		require.True(t, ok)
		require.NotNil(t, m)
		require.False(t, m.Timestamp.IsZero())

		m.Timestamp = time.Time{}
		assert.Equal(t, &model.Metricset{
			Name: "service_destination",
			Metadata: model.Metadata{
				Service: model.Service{Name: "service", Agent: model.Agent{Name: "agent"}},
			},
			Event: model.MetricsetEventCategorization{
				Outcome: "success",
			},
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: newString("destination3")},
			},
			Samples: []model.Sample{
				{Name: "span.destination.service.response_time.count", Value: 1.0},
				{Name: "span.destination.service.response_time.sum.us", Value: 100000.0},
				// No metricset.period is recorded as these metrics are instantanous, not aggregated.
			},
		}, m)
	}
}

func makeSpan(
	serviceName, agentName, destinationServiceResource, outcome string,
	duration time.Duration,
	count float64,
) *model.Span {
	span := &model.Span{
		Metadata:            model.Metadata{Service: model.Service{Name: serviceName, Agent: model.Agent{Name: agentName}}},
		Name:                serviceName + ":" + destinationServiceResource,
		Duration:            duration.Seconds() * 1000,
		RepresentativeCount: count,
		Outcome:             outcome,
	}
	if destinationServiceResource != "" {
		span.DestinationService = &model.DestinationService{
			Resource: &destinationServiceResource,
		}
	}
	return span
}

func makeErrReporter(err error) publish.Reporter {
	return func(context.Context, publish.PendingReq) error { return err }
}

func makeChanReporter(ch chan<- publish.PendingReq) publish.Reporter {
	return func(ctx context.Context, req publish.PendingReq) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- req:
			return nil
		}
	}
}

func expectPublish(t *testing.T, ch <-chan publish.PendingReq) publish.PendingReq {
	t.Helper()
	select {
	case req := <-ch:
		return req
	case <-time.After(time.Second * 5):
		t.Fatal("expected publish")
	}
	panic("unreachable")
}

func newString(s string) *string {
	return &s
}
