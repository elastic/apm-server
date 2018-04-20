package processor

import (
	"fmt"
	"sync"

	"github.com/elastic/beats/libbeat/monitoring"
)

type metricMap struct {
	registry   *monitoring.Registry
	metricKeys []string
	mu         sync.Mutex //guards agent creation
	metrics    map[string]metric
}

func NewMetricMap(r *monitoring.Registry, metricKeys []string) *metricMap {
	m := metricMap{registry: r, metricKeys: metricKeys, metrics: map[string]metric{}}
	return &m
}

func (m *metricMap) Add(agent, key string, count int) {
	if val := m.val(agent, key); val != nil {
		val.Add(int64(count))
	}
}

func (m *metricMap) Inc(agent, key string) {
	if val := m.val(agent, key); val != nil {
		val.Inc()
	}
}

func (m *metricMap) val(agent, key string) *monitoring.Int {
	r, ok := m.metrics[agent]
	if !ok {
		m.mu.Lock()
		defer m.mu.Unlock()
		r, ok = m.metrics[agent]
		if !ok {
			m.metrics[agent] = newMetric(m.registry, agent, m.metricKeys)
			r = m.metrics[agent]
		}
	}
	return r[key]
}

type metric = map[string]*monitoring.Int

func newMetric(r *monitoring.Registry, agent string, keys []string) metric {
	m := metric{}
	for _, k := range keys {
		m[k] = monitoring.NewInt(r, fmt.Sprintf("%s.%s", agent, k))
	}
	return m
}
