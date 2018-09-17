package elasticapm

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/elastic/apm-agent-go/model"
)

// Metrics holds a set of metrics.
type Metrics struct {
	mu      sync.Mutex
	metrics []*model.Metrics
}

func (m *Metrics) reset() {
	m.metrics = m.metrics[:0]
}

// MetricLabel is a name/value pair for labeling metrics.
type MetricLabel struct {
	// Name is the label name.
	Name string

	// Value is the label value.
	Value string
}

// MetricsGatherer provides an interface for gathering metrics.
type MetricsGatherer interface {
	// GatherMetrics gathers metrics and adds them to m.
	//
	// If ctx.Done() is signaled, gathering should be aborted and
	// ctx.Err() returned. If GatherMetrics returns an error, it
	// will be logged, but otherwise there is no effect; the
	// implementation must take care not to leave m in an invalid
	// state due to errors.
	GatherMetrics(ctx context.Context, m *Metrics) error
}

// GatherMetricsFunc is a function type implementing MetricsGatherer.
type GatherMetricsFunc func(context.Context, *Metrics) error

// GatherMetrics calls f(ctx, m).
func (f GatherMetricsFunc) GatherMetrics(ctx context.Context, m *Metrics) error {
	return f(ctx, m)
}

// Add adds a metric with the given name, labels, and value,
// The labels are expected to be sorted lexicographically.
func (m *Metrics) Add(name string, labels []MetricLabel, value float64) {
	m.addMetric(name, labels, model.Metric{Value: value})
}

func (m *Metrics) addMetric(name string, labels []MetricLabel, metric model.Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var metrics *model.Metrics
	results := make([]int, len(m.metrics))
	i := sort.Search(len(m.metrics), func(j int) bool {
		results[j] = compareLabels(m.metrics[j].Labels, labels)
		return results[j] >= 0
	})
	if i < len(results) && results[i] == 0 {
		// labels are equal
		metrics = m.metrics[i]
	} else {
		var modelLabels model.StringMap
		if len(labels) > 0 {
			modelLabels = make(model.StringMap, len(labels))
			for i, l := range labels {
				modelLabels[i] = model.StringMapItem{
					Key: l.Name, Value: l.Value,
				}
			}
		}
		metrics = &model.Metrics{
			Labels:  modelLabels,
			Samples: make(map[string]model.Metric),
		}
		if i == len(results) {
			m.metrics = append(m.metrics, metrics)
		} else {
			m.metrics = append(m.metrics, nil)
			copy(m.metrics[i+1:], m.metrics[i:])
			m.metrics[i] = metrics
		}
	}
	metrics.Samples[name] = metric
}

func compareLabels(a model.StringMap, b []MetricLabel) int {
	na, nb := len(a), len(b)
	n := na
	if na > nb {
		n = nb
	}
	for i := 0; i < n; i++ {
		la, lb := a[i], b[i]
		d := strings.Compare(la.Key, lb.Name)
		if d == 0 {
			d = strings.Compare(la.Value, lb.Value)
		}
		if d != 0 {
			return d
		}
	}
	switch {
	case na < nb:
		return -1
	case na > nb:
		return 1
	}
	return 0
}

func gatherMetrics(ctx context.Context, g MetricsGatherer, m *Metrics, logger Logger) {
	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Debugf("%T.GatherMetrics panicked: %s", g, r)
			}
		}
	}()
	if err := g.GatherMetrics(ctx, m); err != nil {
		if logger != nil && err != context.Canceled {
			logger.Debugf("%T.GatherMetrics failed: %s", g, err)
		}
	}
}
