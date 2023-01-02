// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package baseaggregator

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"
)

// PublishFunc encapsulates the metric publishing function
type PublishFunc func(context.Context, time.Duration) error

// AggregatorConfig defined the common aggregator configuration.
type AggregatorConfig struct {
	// PublishFunc is the function that performs the actual publishing
	// of aggregated events.
	PublishFunc PublishFunc

	// Logger is the logger for logging metrics aggregation/publishing.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger

	// RollUpIntervals are additional MetricsInterval for the aggregator to
	// compute and publish metrics for. Each additional interval is constrained
	// to the same rules as MetricsInterval, and will result in additional
	// memory to be allocated.
	RollUpIntervals []time.Duration

	// Interval is the interval between publishing of aggregated metrics.
	// There may be additional metrics reported at arbitrary times if the
	// aggregation groups fill up.
	Interval time.Duration
}

// Validate validates the aggregator config.
func (cfg AggregatorConfig) Validate() error {
	if cfg.PublishFunc == nil {
		return errors.New("PublishFunc unspecified")
	}
	if cfg.Interval <= 0 {
		return errors.New("Interval unspecified or negative")
	}
	if cfg.Logger == nil {
		return errors.New("Logger unspecified")
	}
	for i, interval := range cfg.RollUpIntervals {
		if interval <= 0 {
			return errors.Errorf("RollUpIntervals[%d]: unspecified or negative", i)
		}
		if interval%cfg.Interval != 0 {
			return errors.Errorf("RollUpIntervals[%d]: interval must be a multiple of MetricsInterval", i)
		}
	}
	return nil
}

// Aggregator contains the basic methods for the metrics aggregators to embed.
type Aggregator struct {
	stopMu    sync.Mutex
	stopping  chan struct{}
	stopped   chan struct{}
	publish   PublishFunc
	Intervals []time.Duration

	config AggregatorConfig
}

// New returns a new base Aggregator.
func New(cfg AggregatorConfig) (*Aggregator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid aggregator config")
	}
	return &Aggregator{
		stopping:  make(chan struct{}),
		stopped:   make(chan struct{}),
		config:    cfg,
		publish:   cfg.PublishFunc,
		Intervals: append([]time.Duration{cfg.Interval}, cfg.RollUpIntervals...),
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
	var ticks uint64
	for !stop {
		select {
		case <-a.stopping:
			stop = true
		case <-ticker.C:
			ticks++
		}
		// Publish the metricsets for all configured intervals.
		for _, interval := range a.Intervals {
			// Publish $interval MetricSets when:
			//  - ticks * MetricsInterval % $interval == 0.
			//  - Aggregator is stopped.
			if !stop && (ticks*uint64(a.config.Interval))%uint64(interval) != 0 {
				continue
			}
			if err := a.publish(context.Background(), interval); err != nil {
				a.config.Logger.With(logp.Error(err)).Warnf(
					"publishing of %s metrics failed: %s",
					interval.String(), err,
				)
			}
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
