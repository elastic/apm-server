// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package spanmetrics

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/logp"
)

const (
	metricsetName = "service_destination"
)

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// BatchProcessor is a model.BatchProcessor for asynchronously
	// processing metrics documents.
	BatchProcessor model.BatchProcessor

	// MaxGroups is the maximum number of distinct service destination
	// group metrics to store within an aggregation period. Once this
	// number of groups is reached, any new aggregation keys will cause
	// individual metrics documents to be immediately published.
	MaxGroups int

	// Interval is the interval between publishing of aggregated metrics.
	// There may be additional metrics reported at arbitrary times if the
	// aggregation groups fill up.
	Interval time.Duration

	// Logger is the logger for logging metrics aggregation/publishing.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger
}

// Validate validates the aggregator config.
func (config AggregatorConfig) Validate() error {
	if config.BatchProcessor == nil {
		return errors.New("BatchProcessor unspecified")
	}
	if config.MaxGroups <= 0 {
		return errors.New("MaxGroups unspecified or negative")
	}
	if config.Interval <= 0 {
		return errors.New("Interval unspecified or negative")
	}
	return nil
}

// Aggregator aggregates transaction durations, periodically publishing histogram spanMetrics.
type Aggregator struct {
	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}

	config AggregatorConfig

	mu               sync.RWMutex
	active, inactive *metricsBuffer
}

// NewAggregator returns a new Aggregator with the given config.
func NewAggregator(config AggregatorConfig) (*Aggregator, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid aggregator config")
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.SpanMetrics)
	}
	return &Aggregator{
		stopping: make(chan struct{}),
		stopped:  make(chan struct{}),
		config:   config,
		active:   newMetricsBuffer(config.MaxGroups),
		inactive: newMetricsBuffer(config.MaxGroups),
	}, nil
}

// Run runs the Aggregator, periodically publishing and clearing aggregated
// metrics. Run returns when either a fatal error occurs, or the Aggregator's
// Stop method is invoked.
func (a *Aggregator) Run() error {
	ticker := time.NewTicker(a.config.Interval)
	defer ticker.Stop()
	defer func() {
		a.stopMu.Lock()
		defer a.stopMu.Unlock()
		select {
		case <-a.stopped:
		default:
			close(a.stopped)
		}
	}()
	var stop bool
	for !stop {
		select {
		case <-a.stopping:
			stop = true
		case <-ticker.C:
		}
		if err := a.publish(context.Background()); err != nil {
			a.config.Logger.With(logp.Error(err)).Warnf(
				"publishing span metrics failed: %s", err,
			)
		}
	}
	return nil
}

// Stop stops the Aggregator if it is running, waiting for it to flush any
// aggregated metrics and return, or for the context to be cancelled.
//
// After Stop has been called the aggregator cannot be reused, as the Run
// method will always return immediately.
func (a *Aggregator) Stop(ctx context.Context) error {
	a.stopMu.Lock()
	select {
	case <-a.stopped:
	case <-a.stopping:
		// Already stopping/stopped.
	default:
		close(a.stopping)
	}
	a.stopMu.Unlock()

	select {
	case <-a.stopped:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (a *Aggregator) publish(ctx context.Context) error {
	// We hold a.mu only long enough to swap the spanMetrics. This will
	// be blocked by spanMetrics updates, which is OK, as we prefer not
	// to block spanMetrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()
	a.active, a.inactive = a.inactive, a.active
	a.mu.Unlock()

	size := len(a.inactive.m)
	if size == 0 {
		a.config.Logger.Debugf("no span metrics to publish")
		return nil
	}

	now := time.Now()
	batch := make(model.Batch, 0, size)
	for key, metrics := range a.inactive.m {
		metricset := makeMetricset(now, key, metrics, a.config.Interval.Milliseconds())
		batch = append(batch, metricset)
		delete(a.inactive.m, key)
	}
	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all spans contained in "b", adding to it any
// metricsets requiring immediate publication.
//
// This method is expected to be used immediately prior to publishing
// the events.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		if event.Processor != model.SpanProcessor {
			continue
		}
		if metricsetEvent := a.processSpan(&event); metricsetEvent.Metricset != nil {
			*b = append(*b, metricsetEvent)
		}
	}
	return nil
}

func (a *Aggregator) processSpan(event *model.APMEvent) model.APMEvent {
	if event.Span.DestinationService == nil || event.Span.DestinationService.Resource == "" {
		return model.APMEvent{}
	}
	if event.Span.RepresentativeCount <= 0 {
		// RepresentativeCount is zero when the sample rate is unknown.
		// We cannot calculate accurate span metrics without the sample
		// rate, so we don't calculate any at all in this case.
		return model.APMEvent{}
	}

	// For composite spans we use the composite sum duration, which is the sum of
	// pre-aggregated spans and excludes time gaps that are counted in the reported
	// span duration. For non-composite spans we just use the reported span duration.
	count := 1
	duration := event.Event.Duration
	if event.Span.Composite != nil {
		count = event.Span.Composite.Count
		duration = time.Duration(event.Span.Composite.Sum * float64(time.Millisecond))
	}

	key := aggregationKey{
		serviceEnvironment: event.Service.Environment,
		serviceName:        event.Service.Name,
		agentName:          event.Agent.Name,
		outcome:            event.Event.Outcome,
		resource:           event.Span.DestinationService.Resource,
	}
	metrics := spanMetrics{
		count: float64(count) * event.Span.RepresentativeCount,
		sum:   float64(duration.Microseconds()) * event.Span.RepresentativeCount,
	}
	if a.active.storeOrUpdate(key, metrics) {
		return model.APMEvent{}
	}
	return makeMetricset(time.Now(), key, metrics, 0)
}

type metricsBuffer struct {
	maxSize int

	mu sync.RWMutex
	m  map[aggregationKey]spanMetrics
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		m:       make(map[aggregationKey]spanMetrics),
	}
}

func (mb *metricsBuffer) storeOrUpdate(key aggregationKey, value spanMetrics) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	old, ok := mb.m[key]
	if !ok && len(mb.m) == mb.maxSize {
		return false
	}
	mb.m[key] = spanMetrics{count: value.count + old.count, sum: value.sum + old.sum}
	return true
}

type aggregationKey struct {
	// origin
	serviceName        string
	serviceEnvironment string
	agentName          string
	// destination
	resource string
	outcome  string
}

type spanMetrics struct {
	count float64
	sum   float64
}

func makeMetricset(timestamp time.Time, key aggregationKey, metrics spanMetrics, interval int64) model.APMEvent {
	out := model.APMEvent{
		Timestamp: timestamp,
		Agent:     model.Agent{Name: key.agentName},
		Service: model.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
		},
		Event: model.Event{
			Outcome: key.outcome,
		},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name: metricsetName,
			Span: model.MetricsetSpan{
				DestinationService: model.DestinationService{Resource: key.resource},
			},
			Samples: map[string]model.MetricsetSample{
				"span.destination.service.response_time.count": {
					Value: math.Round(metrics.count),
				},
				"span.destination.service.response_time.sum.us": {
					Value: math.Round(metrics.sum),
				},
			},
		},
	}
	if interval > 0 {
		// Only set metricset.period for a positive interval.
		//
		// An interval of zero means the metricset is computed
		// from an instantaneous value, meaning there is no
		// aggregation period.
		out.Metricset.Samples["metricset.period"] = model.MetricsetSample{
			Value: float64(interval),
		}
	}
	return out
}
