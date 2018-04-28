package elasticapm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestTracerMetricsGatherer(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	tracer.AddMetricsGatherer(elasticapm.GatherMetricsFunc(
		func(ctx context.Context, m *elasticapm.Metrics) error {
			m.AddGauge("heap_inuse_bytes", nil, 1)
			m.AddGauge("heap_idle_bytes", nil, 2)
			m.AddCounter("http.request", []elasticapm.MetricLabel{
				{Name: "code", Value: "400"},
				{Name: "path", Value: "/"},
			}, 3)
			m.AddCounter("http.request", []elasticapm.MetricLabel{
				{Name: "code", Value: "200"},
			}, 4)
			return nil
		},
	))
	tracer.SendMetrics(nil)

	payloads := transport.Payloads()
	require.Len(t, payloads, 1)

	metrics := payloads[0].Metrics()
	require.Len(t, metrics, 3)

	assert.NotEmpty(t, metrics[0].Timestamp)
	for i := 1; i < len(metrics); i++ {
		assert.Equal(t, metrics[0].Timestamp, metrics[i].Timestamp)
	}
	for i := 0; i < len(metrics); i++ {
		metrics[i].Timestamp = model.Time{}
	}

	assert.Equal(t, []*model.Metrics{{
		Samples: map[string]model.Metric{
			"heap_inuse_bytes": {Value: newFloat64(1)},
			"heap_idle_bytes":  {Value: newFloat64(2)},
		},
	}, {
		Labels: model.StringMap{{Key: "code", Value: "200"}},
		Samples: map[string]model.Metric{
			"http.request": {Count: newFloat64(4)},
		},
	}, {
		Labels: model.StringMap{
			{Key: "code", Value: "400"},
			{Key: "path", Value: "/"},
		},
		Samples: map[string]model.Metric{
			"http.request": {Count: newFloat64(3)},
		},
	}}, metrics)
}

func newFloat64(f float64) *float64 {
	return &f
}
