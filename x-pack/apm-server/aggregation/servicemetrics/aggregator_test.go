// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicemetrics

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/elastic-agent-libs/logp"
)

func BenchmarkAggregateSpan(b *testing.B) {
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeErrBatchProcessor(nil),
		Interval:       time.Minute,
		MaxGroups:      1000,
	})
	require.NoError(b, err)

	transaction := makeTransaction("test_service", "agent", "test_destination", "trg_type", "trg_name", "success", time.Second, 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = agg.ProcessBatch(context.Background(), &model.Batch{transaction})
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
		Interval:       1 * time.Millisecond,
		MaxGroups:      1000,
	})
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
	for _, in := range inputs {
		wg.Add(1)
		go func(in input) {
			defer wg.Done()
			transaction := makeTransaction(in.serviceName, in.agentName, in.destination, in.targetType, in.targetName, in.outcome, 100*time.Millisecond, in.count)
			batch := model.Batch{transaction}
			for i := 0; i < 100; i++ {
				err := agg.ProcessBatch(context.Background(), &batch)
				require.NoError(t, err)
				assert.Equal(t, model.Batch{transaction}, batch)
			}
		}(in)
	}
	wg.Wait()

	// Start the aggregator after processing to ensure metrics are aggregated deterministically.
	go agg.Run()
	defer agg.Stop(context.Background())

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	serviceAFailureCount := 100
	serviceBFailureCount := 0
	expected := []model.APMEvent{{
		Agent: model.Agent{Name: "java"},
		Service: model.Service{
			Name: "service-A",
		},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service"},
		Transaction: &model.Transaction{
			DurationAggregate: model.AggregateMetric{
				Count: 800,
				Sum:   80000000,
				Min:   0,
				Max:   100000,
			},
			FailureCount: &serviceAFailureCount,
		},
	}, {
		Agent: model.Agent{Name: "python"},
		Service: model.Service{
			Name: "service-B",
		},
		Processor: model.MetricsetProcessor,
		Transaction: &model.Transaction{
			DurationAggregate: model.AggregateMetric{
				Count: 100,
				Sum:   10000000,
				Min:   0,
				Max:   100000,
			},
			FailureCount: &serviceBFailureCount,
		},
		Metricset: &model.Metricset{Name: "service"},
	}}

	assert.Equal(t, len(expected), len(metricsets))
	assert.Equal(t, expected[0], metricsets[0])
	assert.Equal(t, expected[1], metricsets[1])

	assert.ElementsMatch(t, expected, metricsets)

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
		transaction := makeTransaction("service_name", "agent_name", "destination", "trg_type", "trg_name", "success", 100*time.Millisecond, 1)
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

func TestAggregatorMaxGroups(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       10 * time.Millisecond,
		MaxGroups:      4,
		Logger:         logger,
	})
	require.NoError(t, err)

	// The first two transaction groups will not require immediate publication,
	// as we have configured the spanmetrics with a maximum of four buckets.
	batch := make(model.Batch, 2)
	batch[0] = makeTransaction("service1", "agent", "destination1", "trg_type_1", "trg_name_1", "success", 100*time.Millisecond, 1)
	batch[1] = makeTransaction("service2", "agent", "destination2", "trg_type_2", "trg_name_2", "success", 100*time.Millisecond, 1)
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))
	assert.Equal(t, 1, observed.FilterMessage("service metrics groups reached 50% capacity").Len())
	assert.Len(t, observed.TakeAll(), 1)

	// After hitting 50% capacity (two buckets), then subsequent new metrics will
	// be aggregated without span.name.
	batch = append(batch,
		makeTransaction("service3", "agent", "destination3", "trg_type_3", "trg_name_3", "success", 100*time.Millisecond, 1),
		makeTransaction("service4", "agent", "destination4", "trg_type_4", "trg_name_4", "success", 100*time.Millisecond, 1),
	)
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))
	assert.Equal(t, 1, observed.FilterMessage("service metrics groups reached 100% capacity").Len())
	assert.Len(t, observed.TakeAll(), 1)

	// After hitting 100% capacity (four buckets), then subsequent new metrics will
	// return single-event metricsets for immediate publication.
	for i := 0; i < 2; i++ {
		batch = append(batch, makeTransaction("service5", "agent", "destination5", "trg_type_5", "trg_name_5", "success", 100*time.Millisecond, 1))
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metricsets := batchMetricsets(t, batch)
	assert.Len(t, metricsets, 2)

	for _, m := range metricsets {
		failureCount := 0
		assert.Equal(t, model.APMEvent{
			Agent: model.Agent{
				Name: "agent",
			},
			Service: model.Service{
				Name: "service5",
			},
			Processor: model.MetricsetProcessor,
			Transaction: &model.Transaction{
				DurationAggregate: model.AggregateMetric{
					Count: 1,
					Sum:   100000,
					Min:   100000,
					Max:   100000,
				},
				FailureCount: &failureCount,
			},
			Metricset: &model.Metricset{Name: "service"},
		}, m)
	}
}

func makeTransaction(
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
		Processor: model.TransactionProcessor,
		Transaction: &model.Transaction{
			Name:                serviceName + ":" + destinationServiceResource,
			RepresentativeCount: count,
		},
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
