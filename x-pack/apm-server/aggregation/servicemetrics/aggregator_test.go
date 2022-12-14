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

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/model"
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
	batches := make(chan model.Batch, 1)
	agg, err := NewAggregator(AggregatorConfig{
		BatchProcessor: makeChanBatchProcessor(batches),
		Interval:       1 * time.Millisecond,
		MaxGroups:      1000,
	})
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

	batch := expectBatch(t, batches)
	metricsets := batchMetricsets(t, batch)
	expected := []model.APMEvent{{
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service", DocCount: 6},
		Service:   model.Service{Name: "backend"},
		Agent:     model.Agent{Name: "java"},
		Transaction: &model.Transaction{
			Type: "request",
			DurationSummary: model.SummaryMetric{
				Count: 6,
				Sum:   6000, // 6ms in micros
			},
			SuccessCount: model.SummaryMetric{
				Count: 5,
				Sum:   2,
			},
		},
	}, {
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service", DocCount: 1},
		Service:   model.Service{Name: "backend"},
		Agent:     model.Agent{Name: "go"},
		Transaction: &model.Transaction{
			Type: "request",
			DurationSummary: model.SummaryMetric{
				Count: 1,
				Sum:   1000, // 1ms in micros
			},
		},
	}, {
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service", DocCount: 1},
		Service:   model.Service{Name: "backend"},
		Agent:     model.Agent{Name: "go"},
		Transaction: &model.Transaction{
			Type: "background",
			DurationSummary: model.SummaryMetric{
				Count: 1,
				Sum:   1000, // 1ms in micros
			},
		},
	}, {
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service", DocCount: 1},
		Service:   model.Service{Name: "frontend"},
		Agent:     model.Agent{Name: "rum-js"},
		Transaction: &model.Transaction{
			Type: "page-load",
			DurationSummary: model.SummaryMetric{
				Count: 1,
				Sum:   1000, // 1ms in micros
			},
		},
	}, {
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{Name: "service", DocCount: 1},
		Service:   model.Service{Name: "frontend", Environment: "staging"},
		Agent:     model.Agent{Name: "rum-js"},
		Transaction: &model.Transaction{
			Type: "page-load",
			DurationSummary: model.SummaryMetric{
				Count: 1,
				Sum:   1000, // 1ms in micros
			},
		},
	}}

	assert.Equal(t, len(expected), len(metricsets))
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

	// The first four groups will not require immediate publication,
	// as we have configured a maximum of four buckets. Log messages
	// are produced after 50% and 100% of capacity are reached.
	batch := make(model.Batch, 2)
	batch[0] = makeTransaction("service1", "agent", "tx_type", "success", 100*time.Millisecond, 1)
	batch[1] = makeTransaction("service2", "agent", "tx_type", "success", 100*time.Millisecond, 1)
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))
	assert.Equal(t, 1, observed.FilterMessage("service metrics groups reached 50% capacity").Len())
	assert.Len(t, observed.TakeAll(), 1)

	batch = append(batch,
		makeTransaction("service3", "agent", "tx_type", "success", 100*time.Millisecond, 1),
		makeTransaction("service4", "agent", "tx_type", "success", 100*time.Millisecond, 1),
	)
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)
	assert.Empty(t, batchMetricsets(t, batch))
	assert.Equal(t, 1, observed.FilterMessage("service metrics groups reached 100% capacity").Len())
	assert.Len(t, observed.TakeAll(), 1)

	// After hitting 100% capacity (four buckets), then subsequent new metrics will
	// return single-event metricsets for immediate publication.
	for i := 0; i < 2; i++ {
		batch = append(batch, makeTransaction("service5", "agent", "tx_type", "success", 100*time.Millisecond, 1))
	}
	err = agg.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	metricsets := batchMetricsets(t, batch)
	assert.Len(t, metricsets, 2)

	for _, m := range metricsets {
		assert.Equal(t, model.APMEvent{
			Agent: model.Agent{
				Name: "agent",
			},
			Service: model.Service{
				Name: "service5",
			},
			Processor: model.MetricsetProcessor,
			Transaction: &model.Transaction{
				Type: "tx_type",
				DurationSummary: model.SummaryMetric{
					Count: 1,
					Sum:   100000,
				},
				SuccessCount: model.SummaryMetric{
					Count: 1,
					Sum:   1,
				},
			},
			Metricset: &model.Metricset{Name: "service", DocCount: 1},
		}, m)
	}
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
