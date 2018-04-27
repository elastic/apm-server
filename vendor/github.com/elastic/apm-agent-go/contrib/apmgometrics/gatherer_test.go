package apmgometrics_test

import (
	"testing"

	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmgometrics"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestGatherer(t *testing.T) {
	r := metrics.NewRegistry()
	httpReqsTotal := metrics.GetOrRegisterCounter("http.requests_total", r)
	httpReqsInflight := metrics.GetOrRegisterGauge("http.requests_inflight", r)
	httpReqsTotal.Inc(123)
	httpReqsInflight.Update(10)

	g := apmgometrics.Wrap(r)
	metrics := gatherMetrics(g)

	assert.Equal(t, []*model.Metrics{{
		Samples: map[string]model.Metric{
			"http.requests_total": {
				// go-metrics's counters are gauges
				// by our definition, hence the Value
				// field is set and Count is not.
				Value: newFloat64(123),
			},
			"http.requests_inflight": {
				Value: newFloat64(10),
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
	for _, m := range metrics {
		m.Timestamp = model.Time{}
	}
	return metrics
}

func newFloat64(v float64) *float64 {
	return &v
}
