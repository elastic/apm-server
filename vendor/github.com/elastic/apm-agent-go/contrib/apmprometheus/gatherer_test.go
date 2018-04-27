package apmprometheus_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmprometheus"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestGoCollector(t *testing.T) {
	g := apmprometheus.Wrap(prometheus.DefaultGatherer)
	metrics := gatherMetrics(g)
	require.Len(t, metrics, 1)
	assert.Nil(t, metrics[0].Labels)

	assert.Contains(t, metrics[0].Samples, "go_memstats_alloc_bytes")
	assert.Contains(t, metrics[0].Samples, "go_memstats_alloc_bytes_total")
	assert.NotNil(t, metrics[0].Samples["go_memstats_alloc_bytes"].Value)
	assert.NotNil(t, metrics[0].Samples["go_memstats_alloc_bytes_total"].Count)
}

func TestLabels(t *testing.T) {
	r := prometheus.NewRegistry()
	httpReqsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "."},
		[]string{"code", "method"},
	)
	httpReqsInflight := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "http_requests_inflight", Help: "."},
		[]string{"code", "method"},
	)
	r.MustRegister(httpReqsTotal)
	r.MustRegister(httpReqsInflight)

	httpReqsTotal.WithLabelValues("404", "GET").Inc()
	httpReqsTotal.WithLabelValues("200", "PUT").Inc()
	httpReqsTotal.WithLabelValues("200", "GET").Add(123)
	httpReqsInflight.WithLabelValues("200", "GET").Set(10)

	g := apmprometheus.Wrap(r)
	metrics := gatherMetrics(g)

	assert.Equal(t, []*model.Metrics{{
		Labels: model.StringMap{
			{Key: "code", Value: "200"},
			{Key: "method", Value: "GET"},
		},
		Samples: map[string]model.Metric{
			"http_requests_total": {
				Count: newFloat64(123),
			},
			"http_requests_inflight": {
				Value: newFloat64(10),
			},
		},
	}, {
		Labels: model.StringMap{
			{Key: "code", Value: "200"},
			{Key: "method", Value: "PUT"},
		},
		Samples: map[string]model.Metric{
			"http_requests_total": {
				Count: newFloat64(1),
			},
		},
	}, {
		Labels: model.StringMap{
			{Key: "code", Value: "404"},
			{Key: "method", Value: "GET"},
		},
		Samples: map[string]model.Metric{
			"http_requests_total": {
				Count: newFloat64(1),
			},
		},
	}}, metrics)
}

func gatherMetrics(g elasticapm.MetricsGatherer) []*model.Metrics {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()
	tracer.AddMetricsGatherer(g)
	tracer.SendMetrics(nil)
	metrics := transport.Payloads()[0].Metrics()
	for _, s := range metrics {
		s.Timestamp = model.Time{}
	}
	return metrics
}

func newFloat64(v float64) *float64 {
	return &v
}
