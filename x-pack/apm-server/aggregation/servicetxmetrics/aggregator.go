// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicetxmetrics

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
	"github.com/elastic/go-hdrhistogram"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/baseaggregator"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/interval"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/labels"
)

const (
	metricsetName       = "service_transaction"
	overflowServiceName = "_other"

	minDuration time.Duration = 0
	maxDuration time.Duration = time.Hour

	histogramCountScale = 1000
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

	// MaxGroups is the maximum number of distinct service transaction metrics
	// to store within an aggregation period. Once this number of groups is
	// reached, any new aggregation keys will be aggregated in a dedicated
	// service group identified by `_other`.
	MaxGroups int

	// HDRHistogramSignificantFigures is the number of significant figures
	// to maintain in the HDR Histograms. HDRHistogramSignificantFigures
	// must be in the range [1,5].
	HDRHistogramSignificantFigures int
}

// Validate validates the aggregator config.
func (config AggregatorConfig) Validate() error {
	if config.BatchProcessor == nil {
		return errors.New("BatchProcessor unspecified")
	}
	if config.MaxGroups <= 0 {
		return errors.New("MaxGroups unspecified or negative")
	}
	if n := config.HDRHistogramSignificantFigures; n < 1 || n > 5 {
		return errors.Errorf("HDRHistogramSignificantFigures (%d) outside range [1,5]", n)
	}
	return nil
}

// Aggregator aggregates service latency and throughput, periodically publishing service transaction metrics.
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
		config.Logger = logp.NewLogger(logs.ServiceTransactionMetrics)
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
		aggregator.active[interval] = newMetricsBuffer(config.MaxGroups, config.HDRHistogramSignificantFigures)
		aggregator.inactive[interval] = newMetricsBuffer(config.MaxGroups, config.HDRHistogramSignificantFigures)
	}
	return &aggregator, nil
}

func (a *Aggregator) publish(ctx context.Context, period time.Duration) error {
	// We hold a.mu only long enough to swap the serviceTxMetrics. This will
	// be blocked by serviceTxMetrics updates, which is OK, as we prefer not
	// to block serviceTxMetrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()

	// EXPLAIN: We swap active <-> inactive, so that we're only working on the
	// inactive property while publish is running. `a.active` is the buffer that
	// receives/stores/updates the metricsets, once swapped, we're working on the
	// `a.inactive` which we're going to process and publish.
	current := a.active[period]
	a.active[period], a.inactive[period] = a.inactive[period], current
	a.mu.Unlock()

	size := len(current.m)
	if size == 0 {
		a.config.Logger.Debugf("no service transaction metrics to publish")
		return nil
	}

	intervalStr := interval.FormatDuration(period)
	batch := make(model.Batch, 0, size)
	for key, metrics := range current.m {
		for _, entry := range metrics {
			totalCount, counts, values := entry.serviceTxMetrics.histogramBuckets()
			// Record the metricset interval as metricset.interval.
			m := makeMetricset(entry.aggregationKey, entry.serviceTxMetrics, totalCount, counts, values, intervalStr)
			batch = append(batch, m)
			entry.histogram.Reset()
		}
		delete(current.m, key)
	}
	if current.other != nil {
		entry := current.other
		totalCount, counts, values := entry.serviceTxMetrics.histogramBuckets()
		// Record the metricset interval as metricset.interval.
		m := makeMetricset(entry.aggregationKey, entry.serviceTxMetrics, totalCount, counts, values, intervalStr)
		m.Metricset.Samples = append(m.Metricset.Samples, model.MetricsetSample{
			Name:  "service_transaction.aggregation.overflow_count",
			Value: float64(current.otherCardinalityEstimator.Estimate()),
		})
		batch = append(batch, m)
		entry.histogram.Reset()
		current.other = nil
		current.otherCardinalityEstimator = nil
	}
	current.entries = 0
	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all service latency metrics.
//
// To contain cardinality of the aggregated metrics the following
// limits are considered:
//
//   - MaxGroups: Limits the total number of services that the
//     service transaction metrics aggregator produces. Once this limit is
//     breached the metrics are aggregated in a dedicated bucket
//     with `service.name` as `_other`.
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
	for _, interval := range a.Intervals {
		key := makeAggregationKey(event, interval)
		metrics := makeServiceTxMetrics(event)
		a.active[interval].storeOrUpdate(key, metrics, interval, a.config.Logger)
	}
}

type metricsBuffer struct {
	mu                        sync.RWMutex
	m                         map[uint64][]*metricsMapEntry
	other                     *metricsMapEntry
	otherCardinalityEstimator *hyperloglog.Sketch
	space                     []metricsMapEntry
	entries                   int

	maxSize            int
	significantFigures int
}

func newMetricsBuffer(maxSize int, significantFigures int) *metricsBuffer {
	return &metricsBuffer{
		maxSize:            maxSize,
		significantFigures: significantFigures,
		// keep one reserved entry for overflow bucket
		space: make([]metricsMapEntry, maxSize+1),
		m:     make(map[uint64][]*metricsMapEntry),
	}
}

type metricsMapEntry struct {
	aggregationKey
	serviceTxMetrics
}

func (mb *metricsBuffer) storeOrUpdate(
	key aggregationKey,
	metrics serviceTxMetrics,
	interval time.Duration,
	logger *logp.Logger,
) {
	// hash does not use the serviceTxMetrics so it is safe to call concurrently.
	hash := key.hash()

	// Full lock because serviceTxMetrics cannot be updated atomically.
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
		entry.serviceTxMetrics = serviceTxMetrics{
			transactionDuration: entry.transactionDuration + metrics.transactionDuration,
			transactionCount:    entry.transactionCount + metrics.transactionCount,
			failureCount:        entry.failureCount + metrics.failureCount,
			successCount:        entry.successCount + metrics.successCount,
			histogram:           entry.histogram,
		}
		entry.recordDuration(time.Duration(metrics.transactionDuration), metrics.transactionCount)
		return
	}
	if mb.entries >= mb.maxSize {
		logger.Warnf(`
Service aggregation group limit of %d reached, new metric documents will be grouped
under a dedicated bucket identified by service name '%s'.`[1:], mb.maxSize, overflowServiceName)
		mb.other = &mb.space[len(mb.space)-1]
		mb.otherCardinalityEstimator = hyperloglog.New14()
		mb.otherCardinalityEstimator.InsertHash(hash)
		entry = mb.other
		key = makeOverflowAggregationKey(key, interval)
	} else {
		entry = &mb.space[mb.entries]
		mb.m[hash] = append(entries, entry)
		mb.entries++
	}
	entry.aggregationKey = key
	entry.serviceTxMetrics = serviceTxMetrics{
		transactionDuration: metrics.transactionDuration,
		transactionCount:    metrics.transactionCount,
		failureCount:        metrics.failureCount,
		successCount:        metrics.successCount,
	}
	if entry.serviceTxMetrics.histogram == nil {
		entry.serviceTxMetrics.histogram = hdrhistogram.New(
			minDuration.Microseconds(),
			maxDuration.Microseconds(),
			mb.significantFigures,
		)
	}
	entry.recordDuration(time.Duration(metrics.transactionDuration), metrics.transactionCount)
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
	h.WriteString(k.serviceLanguageName)
	h.WriteString(k.transactionType)
	return h.Sum64()
}

func (k *aggregationKey) equal(key aggregationKey) bool {
	return k.comparable == key.comparable &&
		k.AggregatedGlobalLabels.Equals(&key.AggregatedGlobalLabels)
}

type comparable struct {
	timestamp time.Time

	agentName           string
	serviceName         string
	serviceEnvironment  string
	serviceLanguageName string
	transactionType     string
}

func makeAggregationKey(event *model.APMEvent, interval time.Duration) aggregationKey {
	key := aggregationKey{
		comparable: comparable{
			// Group metrics by time interval.
			timestamp: event.Timestamp.Truncate(interval),

			agentName:           event.Agent.Name,
			serviceName:         event.Service.Name,
			serviceEnvironment:  event.Service.Environment,
			serviceLanguageName: event.Service.Language.Name,
			transactionType:     event.Transaction.Type,
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

type serviceTxMetrics struct {
	histogram           *hdrhistogram.Histogram
	transactionDuration float64
	transactionCount    float64
	failureCount        float64
	successCount        float64
}

func (m *serviceTxMetrics) recordDuration(d time.Duration, n float64) {
	count := int64(math.Round(n * histogramCountScale))
	m.histogram.RecordValuesAtomic(d.Microseconds(), count)
}

func (m *serviceTxMetrics) histogramBuckets() (totalCount int64, counts []int64, values []float64) {
	// From https://www.elastic.co/guide/en/elasticsearch/reference/current/histogram.html:
	//
	// "For the High Dynamic Range (HDR) histogram mode, the values array represents
	// fixed upper limits of each bucket interval, and the counts array represents
	// the number of values that are attributed to each interval."
	distribution := m.histogram.Distribution()
	counts = make([]int64, 0, len(distribution))
	values = make([]float64, 0, len(distribution))
	for _, b := range distribution {
		if b.Count <= 0 {
			continue
		}
		count := int64(math.Round(float64(b.Count) / histogramCountScale))
		counts = append(counts, count)
		values = append(values, float64(b.To))
		totalCount += count
	}
	return totalCount, counts, values
}

func makeServiceTxMetrics(event *model.APMEvent) serviceTxMetrics {
	transactionCount := event.Transaction.RepresentativeCount
	metrics := serviceTxMetrics{
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

func makeMetricset(key aggregationKey, metrics serviceTxMetrics, totalCount int64, counts []int64, values []float64, interval string) model.APMEvent {
	metricCount := int64(math.Round(metrics.transactionCount))
	return model.APMEvent{
		Timestamp: key.timestamp,
		Service: model.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
			Language: model.Language{
				Name: key.serviceLanguageName,
			},
		},
		Agent: model.Agent{
			Name: key.agentName,
		},
		Labels:        key.Labels,
		NumericLabels: key.NumericLabels,
		Processor:     model.MetricsetProcessor,
		Metricset: &model.Metricset{
			DocCount: totalCount,
			Name:     metricsetName,
			Interval: interval,
		},
		Transaction: &model.Transaction{
			Type: key.transactionType,
			DurationSummary: model.SummaryMetric{
				Count: metricCount,
				Sum:   float64(time.Duration(math.Round(metrics.transactionDuration)).Microseconds()),
			},
			DurationHistogram: model.Histogram{
				Counts: counts,
				Values: values,
			},
		},
		Event: model.Event{
			SuccessCount: model.SummaryMetric{
				Count: int64(math.Round(metrics.successCount + metrics.failureCount)),
				Sum:   metrics.successCount,
			},
		},
	}
}
