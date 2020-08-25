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
	stopMu           sync.Mutex
	stopping         chan struct{}
	stopped          chan struct{}
	config           AggregatorConfig
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

func (a Aggregator) Run() error {
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
		metricset := makeMetricset(now, key, metrics.count, a.config.Interval.Seconds(), float64(metrics.sum))
		metricsets = append(metricsets, &metricset)
		a.inactive.mu.Lock()
		delete(a.inactive.m, key)
		a.inactive.mu.Unlock()
	}
	a.config.Logger.Debugf("publishing %d metricsets", len(metricsets))
	return a.config.Report(ctx, publish.PendingReq{
		Transformables: metricsets,
		Trace:          true,
	})
}

func (a *Aggregator) ProcessTransformables(in []transform.Transformable) []transform.Transformable {
	out := in
	for _, tf := range in {
		if span, ok := tf.(*model.Span); ok &&
			span.DestinationService != nil &&
			span.DestinationService.Resource != nil &&
			span.RepresentativeCount > 0 {
			key := aggregationKey{
				serviceEnvironment: span.Metadata.Service.Environment,
				serviceName:        span.Metadata.Service.Name,
				resource:           *span.DestinationService.Resource,
			}
			duration := time.Duration(span.Duration * float64(time.Millisecond))
			metrics := spanMetrics{
				count: span.RepresentativeCount,
				sum:   duration.Microseconds(),
			}
			a.active.storeOrUpdate(key, metrics)
		}
	}
	return out
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
}

type spanMetrics struct {
	count float64
	sum   int64
}

func makeMetricset(timestamp time.Time, key aggregationKey, count, interval, sum float64) model.Metricset {
	out := model.Metricset{
		Timestamp: timestamp,
		Metadata: model.Metadata{
			Service: model.Service{
				Name:        key.serviceName,
				Environment: key.serviceEnvironment,
			},
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
				Value: sum,
			},
			{
				Name:  "metricset.period",
				Value: interval,
			},
		},
	}
	return out
}
