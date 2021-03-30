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
	metricsets := make([]*model.Metricset, 0, size)
	for key, metrics := range a.inactive.m {
		metricset := makeMetricset(now, key, metrics, a.config.Interval.Milliseconds())
		metricsets = append(metricsets, &metricset)
		delete(a.inactive.m, key)
	}
	a.config.Logger.Debugf("publishing %d metricsets", len(metricsets))
	return a.config.BatchProcessor.ProcessBatch(ctx, &model.Batch{Metricsets: metricsets})
}

// ProcessBatch aggregates all spans contained in "b", adding to it any
// metricsets requiring immediate publication.
//
// This method is expected to be used immediately prior to publishing
// the events.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, span := range b.Spans {
		if metricset := a.processSpan(span); metricset != nil {
			b.Metricsets = append(b.Metricsets, metricset)
		}
	}
	return nil
}

func (a *Aggregator) processSpan(span *model.Span) *model.Metricset {
	if span.DestinationService == nil || span.DestinationService.Resource == "" {
		return nil
	}
	if span.RepresentativeCount <= 0 {
		// RepresentativeCount is zero when the sample rate is unknown.
		// We cannot calculate accurate span metrics without the sample
		// rate, so we don't calculate any at all in this case.
		return nil
	}

	key := aggregationKey{
		serviceEnvironment: span.Metadata.Service.Environment,
		serviceName:        span.Metadata.Service.Name,
		agentName:          span.Metadata.Service.Agent.Name,
		outcome:            span.Outcome,
		resource:           span.DestinationService.Resource,
	}
	duration := time.Duration(span.Duration * float64(time.Millisecond))
	metrics := spanMetrics{
		count: span.RepresentativeCount,
		sum:   float64(duration.Microseconds()) * span.RepresentativeCount,
	}
	if a.active.storeOrUpdate(key, metrics) {
		return nil
	}
	metricset := makeMetricset(time.Now(), key, metrics, 0)
	return &metricset
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

func makeMetricset(timestamp time.Time, key aggregationKey, metrics spanMetrics, interval int64) model.Metricset {
	out := model.Metricset{
		Timestamp: timestamp,
		Name:      metricsetName,
		Metadata: model.Metadata{
			Service: model.Service{
				Name:        key.serviceName,
				Environment: key.serviceEnvironment,
				Agent:       model.Agent{Name: key.agentName},
			},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: key.outcome,
		},
		Span: model.MetricsetSpan{
			DestinationService: model.DestinationService{Resource: key.resource},
		},
		Samples: []model.Sample{
			{
				Name:  "span.destination.service.response_time.count",
				Value: math.Round(metrics.count),
			},
			{
				Name:  "span.destination.service.response_time.sum.us",
				Value: math.Round(metrics.sum),
			},
		},
	}
	if interval > 0 {
		// Only set metricset.period for a positive interval.
		//
		// An interval of zero means the metricset is computed
		// from an instantaneous value, meaning there is no
		// aggregation period.
		out.Samples = append(out.Samples, model.Sample{
			Name:  "metricset.period",
			Value: float64(interval),
		})
	}
	return out
}
