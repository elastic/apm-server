// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package spanmetrics

import (
	"context"
	"sort"
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
		Report:   makeErrReporter(nil),
		Interval: time.Minute,
	})
	require.NoError(b, err)

	span := makeSpan("test_service", "test_destination", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.ProcessTransformables([]transform.Transformable{span})
		}
	})
}

func TestAggregatorRun(t *testing.T) {
	reqs := make(chan publish.PendingReq, 1)
	agg, err := NewAggregator(AggregatorConfig{
		Report:   makeChanReporter(reqs),
		Interval: 10 * time.Millisecond,
	})
	require.NoError(t, err)

	destinationX := "destination-X"
	destinationZ := "destination-Z"

	go agg.Run()
	defer agg.Stop(context.Background())

	var wg sync.WaitGroup
	wg.Add(6)
	go sendEvents(&wg, agg, 2, "service-A", destinationZ)
	go sendEvents(&wg, agg, 1, "service-A", destinationX)
	go sendEvents(&wg, agg, 1, "service-B", destinationZ)
	go sendEvents(&wg, agg, 1, "service-A", destinationZ)
	go sendEvents(&wg, agg, 0, "service-A", destinationZ)
	go sendEvents(&wg, agg, 1, "service-A", "" /* no destination */)
	wg.Wait()
	req := expectPublish(t, reqs)

	require.Len(t, req.Transformables, 3)
	metricsets := make([]*model.Metricset, len(req.Transformables))
	for i, tf := range req.Transformables {
		metricsets[i] = tf.(*model.Metricset)
	}
	sort.Slice(metricsets, func(i, j int) bool {
		return metricsets[i].Metadata.Service.Name+*metricsets[i].Span.DestinationService.Resource <
			metricsets[j].Metadata.Service.Name+*metricsets[j].Span.DestinationService.Resource
	})

	m := metricsets[0]
	require.NotNil(t, m)
	require.False(t, m.Timestamp.IsZero())
	m.Timestamp = time.Time{}
	assert.Equal(t, &model.Metricset{
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A"},
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationX},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 100.0},
			{Name: "destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, m)

	m = metricsets[1]
	require.NotNil(t, m)
	require.False(t, m.Timestamp.IsZero())
	m.Timestamp = time.Time{}
	assert.Equal(t, &model.Metricset{
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-A"},
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 300.0},
			{Name: "destination.service.response_time.sum.us", Value: 30000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, m)

	m = metricsets[2]
	require.NotNil(t, m)
	require.False(t, m.Timestamp.IsZero())
	m.Timestamp = time.Time{}
	assert.Equal(t, &model.Metricset{
		Metadata: model.Metadata{
			Service: model.Service{Name: "service-B"},
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: &destinationZ},
		},
		Samples: []model.Sample{
			{Name: "destination.service.response_time.count", Value: 100.0},
			{Name: "destination.service.response_time.sum.us", Value: 10000000.0},
			{Name: "metricset.period", Value: 10},
		},
	}, m)

	select {
	case <-reqs:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func sendEvents(wg *sync.WaitGroup, agg *Aggregator, count float64, serviceName string, resource string) {
	defer wg.Done()
	span := makeSpan(serviceName, resource, 100*time.Millisecond, count)
	for i := 0; i < 100; i++ {
		agg.ProcessTransformables([]transform.Transformable{span})
	}
}

func makeSpan(
	serviceName string, destinationServiceResource string,
	duration time.Duration,
	count float64,
) *model.Span {
	span := &model.Span{
		Metadata:            model.Metadata{Service: model.Service{Name: serviceName}},
		Name:                serviceName + ":" + destinationServiceResource,
		Duration:            duration.Seconds() * 1000,
		RepresentativeCount: count,
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
