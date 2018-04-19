package processor

import (
	"fmt"

	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	unknown = "unknown"
)

var (
	agents = []string{"nodejs", "python", "ruby", "", unknown}
)

type reqMetric = map[string]*monitoring.Int

func newReqMetric(r *monitoring.Registry, agent string, keys []string) reqMetric {
	m := reqMetric{}
	for _, k := range keys {
		m[k] = monitoring.NewInt(r, fmt.Sprintf("%s.%s", agent, k))
	}
	return m
}

type metricMap struct {
	registry *monitoring.Registry
	metrics  map[string]reqMetric
}

func NewMetrics(r *monitoring.Registry, metricKeys []string) *metricMap {
	m := metricMap{registry: r, metrics: map[string]reqMetric{}}
	for _, agent := range agents {
		m.metrics[agent] = newReqMetric(r, agent, metricKeys)
	}
	return &m
}

func (m *metricMap) Add(agent, key string, count int) {
	if i := m.getInt(agent, key); i != nil {
		i.Add(int64(count))
	}
}

func (m *metricMap) Inc(agent, key string) {
	if i := m.getInt(agent, key); i != nil {
		i.Inc()
	}
}

func (m *metricMap) getInt(agent, key string) *monitoring.Int {
	r, ok := m.metrics[agent]
	if !ok {
		r = m.metrics[unknown]
	}
	return r[key]
}
