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

	span := makeSpan("test_service", "test_destination", "success", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.ProcessTransformables([]transform.Transformable{span})
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

	go agg.Run()
	defer agg.Stop(context.Background())

	type input struct {
		serviceName string
		destination string
		outcome     string
		count       float64
	}

	destinationX := "destination-X"
	destinationZ := "destination-Z"
	inputs := []input{
		{serviceName: "service-A", destination: destinationZ, outcome: "success", count: 2},
		{serviceName: "service-A", destination: destinationX, outcome: "success", count: 1},
		{serviceName: "service-B", destination: destinationZ, outcome: "success", count: 1},
		{serviceName: "service-A", destination: destinationZ, outcome: "success", count: 1},
		{serviceName: "service-A", destination: destinationZ, outcome: "success", count: 0},
		{serviceName: "service-A", outcome: "success", count: 1}, // no destination
		{serviceName: "service-A", destination: destinationZ, outcome: "failure", count: 1},
	}

	var wg sync.WaitGroup
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			span := makeSpan(in.serviceName, in.destination, in.outcome, 100*time.Millisecond, in.count)
			for i := 0; i < 100; i++ {
				agg.ProcessTransformables([]transform.Transformable{span})
			}
		}(in)
	}
	wg.Wait()

	req := expectPublish(t, reqs)
	metricsets := make([]*model.Metricset, len(req.Transformables))
	for i, tf := range req.Transformables {
		ms := tf.(*model.Metricset)
		require.NotZero(t, ms.Timestamp)
		ms.Timestamp = time.Time{}
		metricsets[i] = ms
	}

	assert.ElementsMatch(t, []*model.Metricset{{
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A"},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationX},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 100.0},
			{Name: "destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A"},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "failure",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 100.0},
			{Name: "destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A"},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 300.0},
			{Name: "destination.service.response_time.sum.us", Value: 30000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, {
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-B"},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: "success",
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 100.0},
			{Name: "destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}}, metricsets)

	select {
	case <-reqs:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func makeSpan(
	serviceName string, destinationServiceResource, outcome string,
	duration time.Duration,
	count float64,
) *model.Span {
	span := &model.Span{
		Metadata:            model.Metadata{Service: model.Service{Name: serviceName}},
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
