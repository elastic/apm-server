// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package txmetrics_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/beats/v7/libbeat/logp"
)

func TestNewAggregatorConfigInvalid(t *testing.T) {
	report := makeErrReporter(nil)

	type test struct {
		config txmetrics.AggregatorConfig
		err    string
	}

	for _, test := range []test{{
		config: txmetrics.AggregatorConfig{},
		err:    "Report unspecified",
	}, {
		config: txmetrics.AggregatorConfig{
			Report: report,
		},
		err: "MaxTransactionGroups unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			Report:               report,
			MaxTransactionGroups: 1,
		},
		err: "MetricsInterval unspecified or negative",
	}, {
		config: txmetrics.AggregatorConfig{
			Report:                         report,
			MaxTransactionGroups:           1,
			MetricsInterval:                time.Nanosecond,
			HDRHistogramSignificantFigures: 6,
		},
		err: "HDRHistogramSignificantFigures (6) outside range [1,5]",
	}} {
		agg, err := txmetrics.NewAggregator(test.config)
		require.Error(t, err)
		require.Nil(t, agg)
		assert.EqualError(t, err, "invalid aggregator config: "+test.err)
	}
}

func TestAggregateTransformablesOverflow(t *testing.T) {
	reqs := make(chan publish.PendingReq, 1)

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         makeChanReporter(reqs),
		MaxTransactionGroups:           2,
		MetricsInterval:                time.Microsecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	// The first two transaction groups will not require immediate publication,
	// as we have configured the txmetrics with a maximum of two buckets.
	var input []transform.Transformable
	for i := 0; i < 10; i++ {
		input = append(input, &model.Transaction{Name: newString("foo")})
		input = append(input, &model.Transaction{Name: newString("bar")})
	}
	output := agg.AggregateTransformables(input)
	assert.Equal(t, input, output)

	// The third transaction group will return a metricset for immediate publication.
	for i := 0; i < 2; i++ {
		input = append(input, &model.Transaction{
			Name:     newString("baz"),
			Duration: float64(time.Minute / time.Millisecond),
		})
	}
	output = agg.AggregateTransformables(input)
	assert.Len(t, output, len(input)+2)
	assert.Equal(t, input, output[:len(input)])

	for _, tf := range output[len(input):] {
		m, ok := tf.(*model.Metricset)
		require.True(t, ok)
		require.NotNil(t, m)
		require.False(t, m.Timestamp.IsZero())

		m.Timestamp = time.Time{}
		assert.Equal(t, &model.Metricset{
			Metadata: model.Metadata{},
			Transaction: model.MetricsetTransaction{
				Name: "baz",
				Root: true,
			},
			Samples: []model.Sample{{
				Name:   "transaction.duration.histogram",
				Counts: []int64{1},
				Values: []float64{float64(time.Minute / time.Microsecond)},
			}},
		}, m)
	}
}

func TestAggregatorRun(t *testing.T) {
	reqs := make(chan publish.PendingReq, 1)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         makeChanReporter(reqs),
		MaxTransactionGroups:           2,
		MetricsInterval:                10 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
	})
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
		metricset := agg.AggregateTransaction(&model.Transaction{Name: newString("T-1000")})
		require.Nil(t, metricset)
	}
	for i := 0; i < 800; i++ {
		metricset := agg.AggregateTransaction(&model.Transaction{Name: newString("T-800")})
		require.Nil(t, metricset)
	}

	stopAggregator := runAggregator(agg)
	defer stopAggregator()

	req := expectPublish(t, reqs)
	require.Len(t, req.Transformables, 2)
	metricsets := make([]*model.Metricset, len(req.Transformables))
	for i, tf := range req.Transformables {
		metricsets[i] = tf.(*model.Metricset)
	}
	sort.Slice(metricsets, func(i, j int) bool {
		return metricsets[i].Transaction.Name < metricsets[j].Transaction.Name
	})

	assert.Equal(t, "T-1000", metricsets[0].Transaction.Name)
	assert.Equal(t, []int64{1000}, metricsets[0].Samples[0].Counts)
	assert.Equal(t, "T-800", metricsets[1].Transaction.Name)
	assert.Equal(t, []int64{800}, metricsets[1].Samples[0].Counts)

	select {
	case <-reqs:
		t.Fatal("unexpected publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAggregatorRunPublishErrors(t *testing.T) {
	reqs := make(chan publish.PendingReq, 1)
	chanReporter := makeChanReporter(reqs)
	reportErr := errors.New("report failed")
	report := func(ctx context.Context, req publish.PendingReq) error {
		if err := chanReporter(ctx, req); err != nil {
			return err
		}
		return reportErr
	}

	core, observed := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("foo", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         report,
		MaxTransactionGroups:           2,
		MetricsInterval:                10 * time.Millisecond,
		HDRHistogramSignificantFigures: 1,
		Logger:                         logger,
	})
	require.NoError(t, err)

	stopAggregator := runAggregator(agg)
	defer stopAggregator()

	for i := 0; i < 2; i++ {
		metricset := agg.AggregateTransaction(&model.Transaction{Name: newString("T-1000")})
		require.Nil(t, metricset)
		expectPublish(t, reqs)
	}

	// Wait for aggregator to stop before checking logs, to ensure we don't race with logging.
	stopAggregator()

	logs := observed.FilterMessageSnippet("report failed").All()
	assert.Len(t, logs, 2)
	for _, record := range logs {
		require.Len(t, record.Context, 1)
		assert.Equal(t, "error", record.Context[0].Key)
		assert.Equal(t, reportErr, record.Context[0].Interface)
	}
}

func TestHDRHistogramSignificantFigures(t *testing.T) {
	testHDRHistogramSignificantFigures(t, 1)
	testHDRHistogramSignificantFigures(t, 2)
	testHDRHistogramSignificantFigures(t, 3)
	testHDRHistogramSignificantFigures(t, 4)
	testHDRHistogramSignificantFigures(t, 5)
}

func testHDRHistogramSignificantFigures(t *testing.T, sigfigs int) {
	t.Run(fmt.Sprintf("%d_sigfigs", sigfigs), func(t *testing.T) {
		reqs := make(chan publish.PendingReq, 1)
		agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
			Report:                         makeChanReporter(reqs),
			MaxTransactionGroups:           2,
			MetricsInterval:                10 * time.Millisecond,
			HDRHistogramSignificantFigures: sigfigs,
		})
		require.NoError(t, err)

		durationMillis := func(d time.Duration) float64 {
			return float64(d) / float64(time.Millisecond)
		}

		// The following values will be recorded in either 1, 2, 3, 4, or 5
		// buckets according to the configured number of significant figures.
		for _, duration := range []time.Duration{
			100000 * time.Microsecond,
			101000 * time.Microsecond,
			101100 * time.Microsecond,
			101110 * time.Microsecond,
			101111 * time.Microsecond,
		} {
			metricset := agg.AggregateTransaction(&model.Transaction{
				Name:     newString("T-1000"),
				Duration: durationMillis(duration),
			})
			require.Nil(t, metricset)
		}

		stopAggregator := runAggregator(agg)
		defer stopAggregator()

		req := expectPublish(t, reqs)
		require.Len(t, req.Transformables, 1)

		metricset := req.Transformables[0].(*model.Metricset)
		require.Len(t, metricset.Samples, 1)
		assert.Len(t, metricset.Samples[0].Counts, len(metricset.Samples[0].Values))
		assert.Len(t, metricset.Samples[0].Counts, sigfigs)
	})
}

func BenchmarkAggregateTransaction(b *testing.B) {
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         makeErrReporter(nil),
		MaxTransactionGroups:           1000,
		MetricsInterval:                time.Minute,
		HDRHistogramSignificantFigures: 2,
	})
	require.NoError(b, err)

	tx := &model.Transaction{
		Name:     newString("T-1000"),
		Duration: 1,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agg.AggregateTransaction(tx)
		}
	})
}

func runAggregator(agg *txmetrics.Aggregator) func() error {
	done := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(done)
		done <- agg.Run(ctx)
	}()
	return func() error {
		cancel()
		return <-done
	}
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
	select {
	case req := <-ch:
		return req
	case <-time.After(time.Second):
		t.Fatal("expected publish")
	}
	panic("unreachable")
}

func newString(s string) *string {
	return &s
}
