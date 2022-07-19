// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicemetrics

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	metricsetName = "service"
)

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// BatchProcessor is a model.BatchProcessor for asynchronously
	// processing metrics documents.
	BatchProcessor model.BatchProcessor

	// MaxGroups is the maximum number of distinct service metrics to store within an aggregation period.
	// Once this number of groups is reached, any new aggregation keys will cause
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

// Aggregator aggregates service latency and throughput, periodically publishing service metrics.
type Aggregator struct {
	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}

	config AggregatorConfig

	mu sync.RWMutex
	// These two metricsBuffer are set to the same size and act as buffers
	// for caching and then publishing the metrics as batches.
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
				"publishing service metrics failed: %s", err,
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
	// We hold a.mu only long enough to swap the serviceMetrics. This will
	// be blocked by serviceMetrics updates, which is OK, as we prefer not
	// to block serviceMetrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()

	// EXPLAIN: We swap active <-> inactive, so that we're only working on the
	// inactive property while publish is running. `a.active` is the buffer that
	// receives/stores/updates the metricsets, once swapped, we're working on the
	// `a.inactive` which we're going to process and publish.
	a.active, a.inactive = a.inactive, a.active
	a.mu.Unlock()

	size := len(a.inactive.m)
	if size == 0 {
		a.config.Logger.Debugf("no service metrics to publish")
		return nil
	}

	batch := make(model.Batch, 0, size)
	for key, metrics := range a.inactive.m {
		metricset := makeMetricset(key, metrics)
		batch = append(batch, metricset)
		delete(a.inactive.m, key)
	}
	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all service latency metrics
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		tx := event.Transaction
		if event.Processor == model.TransactionProcessor && tx != nil {
			if msEvent := a.processTransaction(&event); msEvent.Metricset != nil {
				*b = append(*b, msEvent)
			}
		}
	}
	return nil
}

func (a *Aggregator) processTransaction(event *model.APMEvent) model.APMEvent {
	if event.Transaction == nil {
		return model.APMEvent{}
	}

	// For composite spans we use the composite sum duration, which is the sum of
	// pre-aggregated spans and excludes time gaps that are counted in the reported
	// span duration. For non-composite spans we just use the reported span duration.
	duration := event.Event.Duration.Microseconds()

	key := makeAggregationKey(event, a.config.Interval)

	metrics := serviceMetrics{
		count:       1,
		sum:         duration,
		min:         duration,
		max:         duration,
		sumFailures: 0,
		agentName:   event.Agent.Name,
	}
	if a.active.storeOrUpdate(key, metrics, event, a.config.Logger) {
		return model.APMEvent{}
	}
	return makeMetricset(key, metrics)
}

type metricsBuffer struct {
	maxSize int

	mu sync.RWMutex
	m  map[aggregationKey]serviceMetrics
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		m:       make(map[aggregationKey]serviceMetrics),
	}
}

func (mb *metricsBuffer) storeOrUpdate(key aggregationKey, value serviceMetrics, event *model.APMEvent, logger *logp.Logger) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	old, ok := mb.m[key]
	if !ok {
		n := len(mb.m)
		half := mb.maxSize / 2
		if !ok {
			switch n {
			case mb.maxSize:
				return false
			case half - 1:
				logger.Warn("service metrics groups reached 50% capacity")
			case mb.maxSize - 1:
				logger.Warn("service metrics groups reached 100% capacity")
			}
		}
	}
	var max = old.max
	var min = old.min
	if value.max > max {
		max = value.max
	}
	if value.min < min {
		min = value.min
	}
	var sumNewFailures = 0
	if event.Event.Outcome == "failure" {
		sumNewFailures = 1
	}
	var agentName = old.agentName
	if old.agentName == "" {
		agentName = event.Agent.Name
	}
	var metrics = serviceMetrics{
		count:       value.count + old.count,
		sum:         value.sum + old.sum,
		max:         max,
		min:         min,
		sumFailures: old.sumFailures + sumNewFailures,
		agentName:   agentName,
	}
	mb.m[key] = metrics
	return true
}

type aggregationKey struct {
	timestamp time.Time

	// origin
	serviceName        string
	serviceEnvironment string
	transactionType    string

	outcome string
}

func makeAggregationKey(event *model.APMEvent, interval time.Duration) aggregationKey {
	var transactionType = ""
	if event.Transaction != nil {
		transactionType = event.Transaction.Type
	} else if event.Span != nil {
		transactionType = event.Span.Type
	}
	return aggregationKey{
		// Group metrics by time interval.
		timestamp: event.Timestamp.Truncate(interval),

		serviceName:        event.Service.Name,
		serviceEnvironment: event.Service.Environment,
		transactionType:    transactionType,
	}
}

type serviceMetrics struct {
	count int64
	sum   int64

	min int64
	max int64

	sumFailures int
	agentName   string
}

func makeMetricset(key aggregationKey, metrics serviceMetrics) model.APMEvent {
	return model.APMEvent{
		Timestamp: key.timestamp,
		Service: model.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
		},
		Event: model.Event{
			Outcome: key.outcome,
		},
		Agent: model.Agent{
			Name: metrics.agentName,
		},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name: metricsetName,
		},
		Transaction: &model.Transaction{
			FailureCount: &metrics.sumFailures,
			DurationAggregate: model.AggregateMetric{
				Count: metrics.count,
				Sum:   metrics.sum,
				Min:   metrics.min,
				Max:   metrics.max,
			},
		},
	}
}
