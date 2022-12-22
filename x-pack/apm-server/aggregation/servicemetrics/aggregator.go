// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicemetrics

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/labels"
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
		config.Logger = logp.NewLogger(logs.ServiceMetrics)
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
		for _, entry := range metrics {
			m := makeMetricset(entry.aggregationKey, entry.serviceMetrics)
			batch = append(batch, m)
		}
		delete(a.inactive.m, key)
	}
	if a.inactive.other != nil {
		entry := a.inactive.other
		m := makeMetricset(entry.aggregationKey, entry.serviceMetrics)
		m.Metricset.Samples = append(m.Metricset.Samples, model.MetricsetSample{
			Name:  "overflow_count",
			Value: float64(a.inactive.otherCardinalityEstimator.Estimate()),
		})
		batch = append(batch, m)
		a.inactive.other = nil
		a.inactive.otherCardinalityEstimator = nil
	}
	a.inactive.entries = 0
	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all service latency metrics.
//
// To contain cardinality of the aggregated metrics the following
// limits are considered:
//
//   - MaxGroups: Limits the total number of services that the
//     service metrics aggregator produces. Once this limit is
//     breached the metrics are aggregated in a dedicated bucket
//     with `service.name` as `other`.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		tx := event.Transaction
		if event.Processor == model.TransactionProcessor && tx != nil {
			a.processTransaction(&event)
		}
	}
	return nil
}

func (a *Aggregator) processTransaction(event *model.APMEvent) {
	if event.Transaction == nil || event.Transaction.RepresentativeCount <= 0 {
		return
	}
	key := makeAggregationKey(event, a.config.Interval)
	metrics := makeServiceMetrics(event)
	a.active.storeOrUpdate(key, metrics, a.config.Logger)
}

type metricsBuffer struct {
	maxSize int

	mu                        sync.RWMutex
	entries                   int
	space                     []metricsMapEntry
	m                         map[uint64][]*metricsMapEntry
	other                     *metricsMapEntry
	otherCardinalityEstimator *hyperloglog.Sketch
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		// keep one reserved entry for overflow bucket
		space: make([]metricsMapEntry, maxSize+1),
		m:     make(map[uint64][]*metricsMapEntry),
	}
}

type metricsMapEntry struct {
	serviceMetrics
	aggregationKey
}

func (mb *metricsBuffer) storeOrUpdate(key aggregationKey, metrics serviceMetrics, logger *logp.Logger) {
	// hash does not use the serviceMetrics so it is safe to call concurrently.
	hash := key.hash()

	// Full lock because serviceMetrics cannot be updated atomically.
	mb.mu.Lock()
	defer mb.mu.Unlock()

	var entry *metricsMapEntry
	entries, ok := mb.m[hash]
	if ok {
		for offset, old := range entries {
			if old.aggregationKey.equal(key) {
				entry = entries[offset]
				break
			}
		}
	}
	if entry == nil && mb.entries >= mb.maxSize && mb.other != nil {
		entry = mb.other
		// axiomhq/hyerloglog uses metrohash but here we are using
		// xxhash. Metrohash has better performance but since we are
		// already calculating xxhash we can use it directly.
		mb.otherCardinalityEstimator.InsertHash(hash)
	}
	if entry != nil {
		entry.serviceMetrics = serviceMetrics{
			transactionDuration: entry.transactionDuration + metrics.transactionDuration,
			transactionCount:    entry.transactionCount + metrics.transactionCount,
			failureCount:        entry.failureCount + metrics.failureCount,
			successCount:        entry.successCount + metrics.successCount,
		}
		return
	}
	if mb.entries >= mb.maxSize {
		mb.other = &mb.space[len(mb.space)-1]
		mb.otherCardinalityEstimator = hyperloglog.New14()
		mb.otherCardinalityEstimator.InsertHash(hash)
		entry = mb.other
		key = makeOverflowAggregationKey(key)
	} else {
		entry = &mb.space[mb.entries]
		mb.m[hash] = append(entries, entry)
		mb.entries++
	}
	entry.aggregationKey = key
	entry.serviceMetrics = serviceMetrics{
		transactionDuration: metrics.transactionDuration,
		transactionCount:    metrics.transactionCount,
		failureCount:        metrics.failureCount,
		successCount:        metrics.successCount,
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
	h.WriteString(k.transactionType)
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
	transactionType    string
}

func makeAggregationKey(event *model.APMEvent, interval time.Duration) aggregationKey {
	key := aggregationKey{
		comparable: comparable{
			// Group metrics by time interval.
			timestamp: event.Timestamp.Truncate(interval),

			agentName:          event.Agent.Name,
			serviceName:        event.Service.Name,
			serviceEnvironment: event.Service.Environment,
			transactionType:    event.Transaction.Type,
		},
	}
	key.AggregatedGlobalLabels.Read(event)
	return key
}

func makeOverflowAggregationKey(oldKey aggregationKey) aggregationKey {
	return aggregationKey{
		comparable: comparable{
			timestamp:   oldKey.timestamp,
			serviceName: "other",
		},
		AggregatedGlobalLabels: oldKey.AggregatedGlobalLabels,
	}
}

type serviceMetrics struct {
	transactionDuration float64
	transactionCount    float64
	failureCount        float64
	successCount        float64
}

func makeServiceMetrics(event *model.APMEvent) serviceMetrics {
	transactionCount := event.Transaction.RepresentativeCount
	metrics := serviceMetrics{
		transactionDuration: transactionCount * float64(event.Event.Duration),
		transactionCount:    transactionCount,
	}
	switch event.Event.Outcome {
	case "failure":
		metrics.failureCount = transactionCount
	case "success":
		metrics.successCount = transactionCount
	}
	return metrics
}

func makeMetricset(key aggregationKey, metrics serviceMetrics) model.APMEvent {
	metricCount := int64(math.Round(metrics.transactionCount))
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
			DocCount: metricCount,
			Name:     metricsetName,
		},
		Transaction: &model.Transaction{
			Type: key.transactionType,
			SuccessCount: model.SummaryMetric{
				Count: int64(math.Round(metrics.successCount + metrics.failureCount)),
				Sum:   metrics.successCount,
			},
			DurationSummary: model.SummaryMetric{
				Count: metricCount,
				Sum:   float64(time.Duration(math.Round(metrics.transactionDuration)).Microseconds()),
			},
		},
	}
}
