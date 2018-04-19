package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/monitoring"
)

func TestNewMetric(t *testing.T) {
	r := monitoring.Default.NewRegistry("apm-server.test")
	metricKeys := []string{"event", "count"}
	agentR, agentN := "ruby", "nodejs"

	metricMap := NewMetricMap(r, metricKeys)
	metricMap.Add(agentR, "event", 15)
	metricMap.Inc(agentR, "count")
	metricMap.Add(agentN, "event", 23)
	metricMap.Add(agentN, "event", 8)
	metricMap.Inc(agentN, "event")

	assert.Equal(t, int64(15), metricMap.metrics[agentR]["event"].Get())
	assert.Equal(t, int64(1), metricMap.metrics[agentR]["count"].Get())

	assert.Equal(t, int64(32), metricMap.metrics[agentN]["event"].Get())
}
