// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package spanmetrics

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/baseaggregator"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/interval"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/labels"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	metricsetName       = "service_destination"
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
	// There may be additional metrics reported at arbitrary times if the
	// aggregation groups fill up.
	Interval time.Duration

	// MaxGroups is the maximum number of distinct service destination
	// group metrics to store within an aggregation period. Once this
	// number of groups is reached, any new aggregation keys will cause
	// individual metrics documents to be immediately published.
	//
	// Some agents continue to send high cardinality span names, e.g.
	// Elasticsearch spans may contain a document ID
	// (see https://github.com/elastic/apm/issues/439). To protect against
	// this, once MaxGroups becomes 50% full then we will stop aggregating
	// on span.name.
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

// Aggregator aggregates transaction durations, periodically publishing histogram spanMetrics.
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
		config.Logger = logp.NewLogger(logs.SpanMetrics)
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
	// We hold a.mu only long enough to swap the spanMetrics. This will
	// be blocked by spanMetrics updates, which is OK, as we prefer not
	// to block spanMetrics updaters. After the lock is released nothing
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
		a.config.Logger.Debugf("no span metrics to publish")
		return nil
	}

	size := current.entries
	if current.other != nil {
		size++
	}

	intervalStr := interval.FormatDuration(period)
	batch := make(model.Batch, 0, size)
	for hash, entries := range current.m {
		for _, entry := range entries {
			event := makeMetricset(entry.aggregationKey, entry.spanMetrics)
			// Record the metricset interval as metricset.interval.
			event.Metricset.Interval = intervalStr
			batch = append(batch, event)
		}
		delete(current.m, hash)
	}
	if current.other != nil {
		entry := current.other
		m := makeMetricset(entry.aggregationKey, entry.spanMetrics)
		m.Metricset.Interval = intervalStr
		count := current.otherCardinalityEstimator.Estimate()
		m.Metricset.Samples = append(m.Metricset.Samples, model.MetricsetSample{
			Name:  "service_destination.aggregation.overflow_count",
			Value: float64(count),
		})
		m.Metricset.DocCount = int64(count)
		batch = append(batch, m)
		current.other = nil
		current.otherCardinalityEstimator = nil
	}
	current.entries = 0
	a.config.Logger.Debugf("%s interval: publishing %d metricsets", period, len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all spans contained in "b", adding to it any
// metricsets requiring immediate publication. It also aggregates transactions
// where transaction.DroppedSpansStats > 0.
//
// This method is expected to be used immediately prior to publishing
// the events.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		if event.Processor == model.SpanProcessor {
			a.processSpan(&event, b)
			continue
		}

		tx := event.Transaction
		if event.Processor == model.TransactionProcessor && tx != nil {
			for _, dss := range tx.DroppedSpansStats {
				a.processDroppedSpanStats(&event, dss, b)
			}
			// NOTE(marclop) The event.Transaction.DroppedSpansStats is unset
			// via the `modelprocessor.DroppedSpansStatsDiscarder` appended just
			// before the Elasticsearch publisher.
			continue
		}
	}
	return nil
}

func (a *Aggregator) processSpan(event *model.APMEvent, b *model.Batch) {
	if event.Span.DestinationService == nil || event.Span.DestinationService.Resource == "" {
		return
	}
	if event.Span.RepresentativeCount <= 0 {
		// RepresentativeCount is zero when the sample rate is unknown.
		// We cannot calculate accurate span metrics without the sample
		// rate, so we don't calculate any at all in this case.
		return
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

	var serviceTargetType, serviceTargetName string
	if event.Service.Target != nil {
		serviceTargetType = event.Service.Target.Type
		serviceTargetName = event.Service.Target.Name
	}
	metrics := spanMetrics{
		count: float64(count) * event.Span.RepresentativeCount,
		sum:   float64(duration) * event.Span.RepresentativeCount,
	}
	for _, interval := range a.Intervals {
		key := makeAggregationKey(
			event,
			event.Span.DestinationService.Resource,
			serviceTargetType,
			serviceTargetName,
			event.Span.Name,
			interval,
		)
		a.active[interval].storeOrUpdate(key, metrics, interval, a.config.Logger)
	}
}

func (a *Aggregator) processDroppedSpanStats(event *model.APMEvent, dss model.DroppedSpanStats, b *model.Batch) {
	representativeCount := event.Transaction.RepresentativeCount
	if representativeCount <= 0 {
		// RepresentativeCount is zero when the sample rate is unknown.
		// We cannot calculate accurate span metrics without the sample
		// rate, so we don't calculate any at all in this case.
		return
	}

	metrics := spanMetrics{
		count: float64(dss.Duration.Count) * representativeCount,
		sum:   float64(dss.Duration.Sum) * representativeCount,
	}
	for _, interval := range a.Intervals {
		key := makeAggregationKey(
			event,
			dss.DestinationServiceResource,
			dss.ServiceTargetType,
			dss.ServiceTargetName,

			// BUG(axw) dropped span statistics do not contain span name.
			// Capturing the service name requires changes to Elastic APM agents.
			"",

			interval,
		)
		a.active[interval].storeOrUpdate(key, metrics, interval, a.config.Logger)
	}
}

type metricsBuffer struct {
	mu sync.RWMutex
	m  map[uint64][]*metricsMapEntry
	// Number of metricsMapEntry in m. May not equal to len(m) in case of hash collision.
	// Does not include overflow bucket.
	entries int

	other                     *metricsMapEntry
	otherCardinalityEstimator *hyperloglog.Sketch

	maxSize int
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		m:       make(map[uint64][]*metricsMapEntry),
	}
}

type metricsMapEntry struct {
	aggregationKey
	spanMetrics
}

func (mb *metricsBuffer) storeOrUpdate(
	key aggregationKey,
	value spanMetrics,
	interval time.Duration,
	logger *logp.Logger,
) {
	// hash does not use the spanMetrics so it is safe to call concurrently.
	hash := key.hash()

	mb.mu.Lock()
	defer mb.mu.Unlock()

	find := func(hash uint64, key aggregationKey) (*metricsMapEntry, bool) {
		// This function should only be called when caller is holding the lock mb.mu.
		// It takes separate hash and key arguments so that hash can be computed
		// before acquiring the lock.
		entries, ok := mb.m[hash]
		if ok {
			for _, old := range entries {
				if old.aggregationKey.equal(key) {
					return old, true
				}
			}
		}
		return nil, false
	}

	old, ok := find(hash, key)
	if !ok {
		half := mb.maxSize / 2
		if mb.entries >= half {
			// To protect against agents that send high cardinality
			// span names, stop aggregating on span.name once the
			// number of groups reaches 50% capacity.
			key.spanName = ""
			hash = key.hash()
			old, ok = find(hash, key)
		}
		if !ok {
			if mb.entries >= mb.maxSize {
				if mb.other == nil {
					logger.Warnf(`
Service aggregation group limit of %d reached, new metric documents will be grouped
under a dedicated bucket identified by service name '%s'.`[1:], mb.maxSize, overflowServiceName)
					mb.otherCardinalityEstimator = hyperloglog.New14()
					mb.other = &metricsMapEntry{aggregationKey: makeOverflowAggregationKey(key, interval)}
				}
				old, ok = mb.other, true
				mb.otherCardinalityEstimator.InsertHash(hash)
			} else {
				mb.m[hash] = append(mb.m[hash], &metricsMapEntry{
					aggregationKey: key,
					spanMetrics:    spanMetrics{count: value.count, sum: value.sum},
				})
				mb.entries++
				return
			}
		}
	}

	old.spanMetrics = spanMetrics{count: value.count + old.count, sum: value.sum + old.sum}
}

type aggregationKey struct {
	labels.AggregatedGlobalLabels
	comparable
}

type comparable struct {
	timestamp time.Time

	// origin
	serviceName        string
	serviceEnvironment string
	agentName          string

	// operation (span)
	spanName string
	outcome  string

	// target
	targetType string
	targetName string

	// destination
	resource string
}

func (k *aggregationKey) hash() uint64 {
	var h xxhash.Digest
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(k.timestamp.UnixNano()))
	h.Write(buf[:])

	k.AggregatedGlobalLabels.Write(&h)
	h.WriteString(k.serviceName)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.agentName)
	h.WriteString(k.spanName)
	h.WriteString(k.outcome)
	h.WriteString(k.targetType)
	h.WriteString(k.targetName)
	h.WriteString(k.resource)
	return h.Sum64()
}

func (k *aggregationKey) equal(key aggregationKey) bool {
	return k.comparable == key.comparable &&
		k.AggregatedGlobalLabels.Equals(&key.AggregatedGlobalLabels)
}

func makeAggregationKey(
	event *model.APMEvent, resource, targetType, targetName, spanName string, interval time.Duration,
) aggregationKey {
	key := aggregationKey{
		comparable: comparable{
			// Group metrics by time interval.
			timestamp: event.Timestamp.Truncate(interval),

			serviceName:        event.Service.Name,
			serviceEnvironment: event.Service.Environment,
			agentName:          event.Agent.Name,

			spanName: spanName,
			outcome:  event.Event.Outcome,

			targetType: targetType,
			targetName: targetName,

			resource: resource,
		},
	}
	key.AggregatedGlobalLabels.Read(event)
	return key
}

func makeOverflowAggregationKey(oldKey aggregationKey, interval time.Duration) aggregationKey {
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

type spanMetrics struct {
	count float64
	sum   float64
}

func makeMetricset(key aggregationKey, metrics spanMetrics) model.APMEvent {
	var target *model.ServiceTarget
	if key.targetName != "" || key.targetType != "" {
		target = &model.ServiceTarget{
			Type: key.targetType,
			Name: key.targetName,
		}
	}
	return model.APMEvent{
		Timestamp: key.timestamp,
		Agent:     model.Agent{Name: key.agentName},
		Service: model.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
			Target:      target,
		},
		Labels:        key.Labels,
		NumericLabels: key.NumericLabels,
		Event: model.Event{
			Outcome: key.outcome,
		},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name:     metricsetName,
			DocCount: int64(math.Round(metrics.count)),
		},
		Span: &model.Span{
			Name: key.spanName,
			DestinationService: &model.DestinationService{
				Resource: key.resource,
				ResponseTime: model.AggregatedDuration{
					Count: int(math.Round(metrics.count)),
					Sum:   time.Duration(math.Round(metrics.sum)),
				},
			},
		},
	}
}
