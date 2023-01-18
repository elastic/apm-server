// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicesummarymetrics

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/baseaggregator"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/interval"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/labels"
)

const (
	metricsetName       = "service_summary"
	overflowServiceName = "_other"
)

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// BatchProcessor is a model.BatchProcessor for asynchronously
	// processing metrics documents.
	BatchProcessor model.BatchProcessor

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
	Interval time.Duration

	// MaxGroups is the maximum number of distinct service summary metrics to
	// store within an aggregation period.
	// Once this number of groups is reached, any new aggregation keys will cause
	// individual metrics documents to be immediately published.
	MaxGroups int
}

// Validate validates the aggregator config.
func (config AggregatorConfig) Validate() error {
	if config.BatchProcessor == nil {
		return errors.New("BatchProcessor unspecified")
	}
	if config.MaxGroups <= 0 {
		return errors.New("MaxGroups unspecified or negative")
	}
	return nil
}

// Aggregator aggregates all events, periodically publishing service summary metrics.
type Aggregator struct {
	*baseaggregator.Aggregator

	config AggregatorConfig

	mu sync.RWMutex
	// These two metricsBuffer are set to the same size and act as buffers
	// for caching and then publishing the metrics as batches.
	active, inactive map[time.Duration]*metricsBuffer
}

// NewAggregator returns a new Aggregator with the given config.
func NewAggregator(config AggregatorConfig) (*Aggregator, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid aggregator config")
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.ServiceSummaryMetrics)
	}
	aggregator := Aggregator{
		config:   config,
		active:   make(map[time.Duration]*metricsBuffer),
		inactive: make(map[time.Duration]*metricsBuffer),
	}
	base, err := baseaggregator.New(baseaggregator.AggregatorConfig{
		PublishFunc:     aggregator.publish, // inject local publish
		Logger:          config.Logger,
		Interval:        config.Interval,
		RollUpIntervals: config.RollUpIntervals,
	})
	if err != nil {
		return nil, err
	}
	aggregator.Aggregator = base
	for _, interval := range aggregator.Intervals {
		aggregator.active[interval] = newMetricsBuffer(config.MaxGroups)
		aggregator.inactive[interval] = newMetricsBuffer(config.MaxGroups)
	}
	return &aggregator, nil
}

func (a *Aggregator) publish(ctx context.Context, period time.Duration) error {
	// We hold a.mu only long enough to swap the serviceSummaryMetrics. This will
	// be blocked by serviceSummaryMetrics updates, which is OK, as we prefer not
	// to block serviceSummaryMetrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()

	// EXPLAIN: We swap active <-> inactive, so that we're only working on the
	// inactive property while publish is running. `a.active` is the buffer that
	// receives/stores/updates the metricsets, once swapped, we're working on the
	// `a.inactive` which we're going to process and publish.
	current := a.active[period]
	a.active[period], a.inactive[period] = a.inactive[period], current
	a.mu.Unlock()

	if current.entries == 0 {
		a.config.Logger.Debugf("no service summary metrics to publish")
		return nil
	}

	size := current.entries
	if current.other != nil {
		size++
	}

	intervalStr := interval.FormatDuration(period)
	batch := make(model.Batch, 0, size)
	for key, metrics := range current.m {
		for _, entry := range metrics {
			m := makeMetricset(*entry, intervalStr)
			batch = append(batch, m)
		}
		delete(current.m, key)
	}
	if current.other != nil {
		m := makeMetricset(*current.other, intervalStr)
		m.Metricset.Samples = append(m.Metricset.Samples, model.MetricsetSample{
			Name:  "service_summary.aggregation.overflow_count",
			Value: float64(current.otherCardinalityEstimator.Estimate()),
		})
		batch = append(batch, m)
	}

	// Clean up everything.
	current.entries = 0
	current.other = nil
	current.otherCardinalityEstimator = nil

	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all service summary metrics.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		// Ignoring spans since they add no value.
		if event.Processor == model.SpanProcessor {
			continue
		}
		a.processEvent(&event)
	}
	return nil
}

func (a *Aggregator) processEvent(event *model.APMEvent) {
	for _, interval := range a.Intervals {
		key := makeAggregationKey(event, interval)
		a.active[interval].storeOrUpdate(key, interval, a.config.Logger)
	}
}

type metricsBuffer struct {
	mu                        sync.RWMutex
	m                         map[uint64][]*aggregationKey
	other                     *aggregationKey
	otherCardinalityEstimator *hyperloglog.Sketch

	// Number of aggregation keys in m, excluding overflow bucket.
	entries int

	maxSize int
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		m:       make(map[uint64][]*aggregationKey),
	}
}

func (mb *metricsBuffer) storeOrUpdate(
	key aggregationKey,
	interval time.Duration,
	logger *logp.Logger,
) {
	hash := key.hash()

	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Search in hash table with separate chaining.
	entries, ok := mb.m[hash]
	if ok {
		ok = false
		for _, old := range entries {
			if old.equal(key) {
				ok = true
			}
		}
	}

	if ok {
		return
	}

	if mb.entries >= mb.maxSize {
		if mb.otherCardinalityEstimator == nil {
			logger.Warnf(`
Service summary aggregation group limit of %d reached, new metric documents will be grouped
under a dedicated bucket identified by service name '%s'.`[1:], mb.maxSize, overflowServiceName)
			key = makeOverflowAggregationKey(interval)
			mb.other = &key
			mb.otherCardinalityEstimator = hyperloglog.New14()
		}
		mb.otherCardinalityEstimator.InsertHash(hash)
	} else {
		mb.m[hash] = append(mb.m[hash], &key)
		mb.entries++
	}
}

type aggregationKey struct {
	labels.AggregatedGlobalLabels
	comparable
}

func (k *aggregationKey) hash() uint64 {
	var h xxhash.Digest
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(k.timestamp.UnixNano()))
	h.Write(buf[:])

	k.AggregatedGlobalLabels.Write(&h)
	h.WriteString(k.agentName)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.serviceName)
	return h.Sum64()
}

func (k *aggregationKey) equal(key aggregationKey) bool {
	return k.comparable == key.comparable &&
		k.AggregatedGlobalLabels.Equals(&key.AggregatedGlobalLabels)
}

type comparable struct {
	timestamp time.Time

	agentName          string
	serviceName        string
	serviceEnvironment string
}

func makeAggregationKey(event *model.APMEvent, interval time.Duration) aggregationKey {
	key := aggregationKey{
		comparable: comparable{
			// Group metrics by time interval.
			timestamp: event.Timestamp.Truncate(interval),

			agentName:          event.Agent.Name,
			serviceName:        event.Service.Name,
			serviceEnvironment: event.Service.Environment,
		},
	}
	key.AggregatedGlobalLabels.Read(event)
	return key
}

func makeOverflowAggregationKey(interval time.Duration) aggregationKey {
	return aggregationKey{
		comparable: comparable{
			// We are using `time.Now` here to align the overflow aggregation to
			// the evaluation time rather than event time. This prevents us from
			// cases of bad timestamps when the server receives some events with
			// old timestamp and these events overflow causing the indexed event
			// to have old timestamp too.
			timestamp:   time.Now().Truncate(interval),
			serviceName: overflowServiceName,
		},
	}
}

func makeMetricset(key aggregationKey, interval string) model.APMEvent {
	return model.APMEvent{
		Timestamp: key.timestamp,
		Service: model.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
		},
		Agent: model.Agent{
			Name: key.agentName,
		},
		Labels:        key.Labels,
		NumericLabels: key.NumericLabels,
		Processor:     model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name:     metricsetName,
			Interval: interval,
		},
	}
}
