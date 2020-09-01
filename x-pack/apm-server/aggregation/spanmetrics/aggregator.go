// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package spanmetrics

import (
	"context"
	"math"
	"sync"
	"time"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	Report   publish.Reporter
	Logger   *logp.Logger
	Interval time.Duration
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
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.SpanMetrics)
	}
	return &Aggregator{
		stopping: make(chan struct{}),
		stopped:  make(chan struct{}),
		config:   config,
		active:   newMetricsBuffer(),
		inactive: newMetricsBuffer(),
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
	metricsets := make([]transform.Transformable, 0, size)
	for key, metrics := range a.inactive.m {
		metricset := makeMetricset(now, key, metrics.count, metrics.sum, a.config.Interval.Milliseconds())
		metricsets = append(metricsets, &metricset)
		delete(a.inactive.m, key)
	}
	a.config.Logger.Debugf("publishing %d metricsets", len(metricsets))
	return a.config.Report(ctx, publish.PendingReq{
		Transformables: metricsets,
		Trace:          true,
	})
}

// ProcessTransformables aggregates all transactions contained in
// "in", returning the input.
//
// This method is expected to be used immediately prior to publishing
// the events.
func (a *Aggregator) ProcessTransformables(in []transform.Transformable) []transform.Transformable {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := in
	for _, tf := range in {
		span, ok := tf.(*model.Span)
		if !ok {
			continue
		}
		a.processSpan(span)
	}
	return out
}

func (a *Aggregator) processSpan(span *model.Span) {
	if span.DestinationService == nil || span.DestinationService.Resource == nil {
		return
	}
	if span.RepresentativeCount <= 0 {
		// RepresentativeCount is zero when the sample rate is unknown.
		// We cannot calculate accurate span metrics without the sample
		// rate, so we don't calculate any at all in this case.
		return
	}

	key := aggregationKey{
		serviceEnvironment: span.Metadata.Service.Environment,
		serviceName:        span.Metadata.Service.Name,
		outcome:            span.Outcome,
		resource:           *span.DestinationService.Resource,
	}
	duration := time.Duration(span.Duration * float64(time.Millisecond))
	metrics := spanMetrics{
		count: span.RepresentativeCount,
		sum:   float64(duration.Microseconds()) * span.RepresentativeCount,
	}
	a.active.storeOrUpdate(key, metrics)
}

type metricsBuffer struct {
	// TODO might need a size cap
	mu sync.RWMutex
	m  map[aggregationKey]spanMetrics
}

func newMetricsBuffer() *metricsBuffer {
	return &metricsBuffer{
		m: make(map[aggregationKey]spanMetrics),
	}
}

func (mb *metricsBuffer) storeOrUpdate(key aggregationKey, value spanMetrics) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	old := mb.m[key]
	mb.m[key] = spanMetrics{count: value.count + old.count, sum: value.sum + old.sum}
}

type aggregationKey struct {
	// origin
	serviceName        string
	serviceEnvironment string
	// destination
	resource string
	outcome  string
}

type spanMetrics struct {
	count float64
	sum   float64
}

func makeMetricset(timestamp time.Time, key aggregationKey, count, sum float64, interval int64) model.Metricset {
	out := model.Metricset{
		Timestamp: timestamp,
		Metadata: model.Metadata{
			Service: model.Service{
				Name:        key.serviceName,
				Environment: key.serviceEnvironment,
			},
		},
		Event: model.MetricsetEventCategorization{
			Outcome: key.outcome,
		},
		Span: model.MetricsetSpan{
			// TODO add span type/subtype?
			DestinationService: model.DestinationService{Resource: &key.resource},
		},
		Samples: []model.Sample{
			{
				Name:  "destination.service.response_time.count",
				Value: math.Round(count),
			},
			{
				Name:  "destination.service.response_time.sum.us",
				Value: math.Round(sum),
			},
			{
				Name:  "metricset.period",
				Value: float64(interval),
			},
		},
	}
	return out
}
