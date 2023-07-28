// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicetxmetrics

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-hdrhistogram"

	"github.com/elastic/apm-data/model/modelpb"
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
	BatchProcessor modelpb.BatchProcessor

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
	config  AggregatorConfig
	metrics *aggregatorMetrics

	mu sync.RWMutex
	// These two metricsBuffer are set to the same size and act as buffers
	// for caching and then publishing the metrics as batches.
	active, inactive map[time.Duration]*metricsBuffer

	histogramPool sync.Pool
}

type aggregatorMetrics struct {
	activeGroups int64
	overflow     int64
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
		metrics:  &aggregatorMetrics{},
		active:   make(map[time.Duration]*metricsBuffer),
		inactive: make(map[time.Duration]*metricsBuffer),
		histogramPool: sync.Pool{New: func() interface{} {
			return hdrhistogram.New(
				minDuration.Microseconds(),
				maxDuration.Microseconds(),
				config.HDRHistogramSignificantFigures,
			)
		}},
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
		aggregator.active[interval] = newMetricsBuffer(config.MaxGroups, &aggregator.histogramPool)
		aggregator.inactive[interval] = newMetricsBuffer(config.MaxGroups, &aggregator.histogramPool)
	}
	return &aggregator, nil
}

// CollectMonitoring may be called to collect monitoring metrics from the
// aggregation. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.aggregation.servicetxmetrics" registry.
func (a *Aggregator) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	activeGroups := int64(atomic.LoadInt64(&a.metrics.activeGroups))
	overflowed := int64(atomic.LoadInt64(&a.metrics.overflow))
	monitoring.ReportInt(V, "active_groups", activeGroups)
	monitoring.ReportInt(V, "overflowed.total", overflowed)
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

	if current.entries == 0 {
		a.config.Logger.Debugf("no service transaction metrics to publish")
		return nil
	}

	size := current.entries
	if current.other != nil {
		size++
	}

	isMetricsPeriod := period == a.config.Interval
	intervalStr := interval.FormatDuration(period)
	batch := make(modelpb.Batch, 0, size)
	for key, metrics := range current.m {
		for _, entry := range metrics {
			// Record the metricset interval as metricset.interval.
			m := makeMetricset(entry.aggregationKey, entry.serviceTxMetrics, intervalStr)
			batch = append(batch, m)
			entry.histogram.Reset()
			a.histogramPool.Put(entry.histogram)
			entry.histogram = nil
		}
		delete(current.m, key)
	}
	if current.other != nil {
		overflowCount := current.otherCardinalityEstimator.Estimate()
		if isMetricsPeriod {
			atomic.AddInt64(&a.metrics.activeGroups, int64(current.entries))
			atomic.AddInt64(&a.metrics.overflow, int64(overflowCount))
		}
		entry := current.other
		// Record the metricset interval as metricset.interval.
		m := makeMetricset(entry.aggregationKey, entry.serviceTxMetrics, intervalStr)
		m.Metricset.Samples = append(m.Metricset.Samples, &modelpb.MetricsetSample{
			Name:  "service_transaction.aggregation.overflow_count",
			Value: float64(overflowCount),
		})
		batch = append(batch, m)
		entry.histogram.Reset()
		a.histogramPool.Put(entry.histogram)
		entry.histogram = nil
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
func (a *Aggregator) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, event := range *b {
		tx := event.Transaction
		if event.Type() == modelpb.TransactionEventType && tx != nil {
			a.processTransaction(event)
		}
	}
	return nil
}

func (a *Aggregator) processTransaction(event *modelpb.APMEvent) {
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

	maxSize int

	histogramPool *sync.Pool
}

func newMetricsBuffer(maxSize int, histogramPool *sync.Pool) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		// keep one reserved entry for overflow bucket
		space:         make([]metricsMapEntry, maxSize+1),
		m:             make(map[uint64][]*metricsMapEntry),
		histogramPool: histogramPool,
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
		entry.recordMetrics(metrics)
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
		histogram: mb.histogramPool.Get().(*hdrhistogram.Histogram),
	}
	entry.recordMetrics(metrics)
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

func makeAggregationKey(event *modelpb.APMEvent, interval time.Duration) aggregationKey {
	key := aggregationKey{
		comparable: comparable{

			agentName:           event.GetAgent().GetName(),
			serviceName:         event.GetService().GetName(),
			serviceEnvironment:  event.GetService().GetEnvironment(),
			serviceLanguageName: event.GetService().GetLanguage().GetName(),
			transactionType:     event.GetTransaction().GetType(),
		},
	}
	if event.Timestamp != nil {
		// Group metrics by time interval.
		key.comparable.timestamp = event.Timestamp.AsTime().Truncate(interval)
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

func (m *serviceTxMetrics) recordMetrics(other serviceTxMetrics) {
	m.transactionCount += other.transactionCount
	m.transactionDuration += other.transactionDuration * other.transactionCount
	m.successCount += other.successCount
	m.failureCount += other.failureCount

	count := int64(math.Round(other.transactionCount * histogramCountScale))
	m.histogram.RecordValuesAtomic(time.Duration(other.transactionDuration).Microseconds(), count)
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

func makeServiceTxMetrics(event *modelpb.APMEvent) serviceTxMetrics {
	transactionCount := event.Transaction.RepresentativeCount
	metrics := serviceTxMetrics{
		transactionDuration: float64(event.Event.Duration.AsDuration()),
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

// makeMetricset creates a metricset with key, metrics, and interval.
// It uses result from histogram for Transaction.DurationSummary and DocCount to avoid discrepancy and UI weirdness.
// Event.SuccessCount will maintain separate counts, which may be different from histogram count,
// but is acceptable since it is used for calculating error ratio.
func makeMetricset(key aggregationKey, metrics serviceTxMetrics, interval string) *modelpb.APMEvent {
	totalCount, counts, values := metrics.histogramBuckets()

	transactionDurationSummary := modelpb.SummaryMetric{
		Count: totalCount,
	}
	for i, v := range values {
		transactionDurationSummary.Sum += v * float64(counts[i])
	}

	var t *timestamppb.Timestamp
	if !key.timestamp.IsZero() {
		t = timestamppb.New(key.timestamp)
	}

	var event *modelpb.Event
	if metrics.successCount != 0 || metrics.failureCount != 0 {
		event = &modelpb.Event{
			SuccessCount: &modelpb.SummaryMetric{
				Count: int64(math.Round(metrics.successCount + metrics.failureCount)),
				Sum:   math.Round(metrics.successCount),
			},
		}
	}

	var agent *modelpb.Agent
	if key.agentName != "" {
		agent = &modelpb.Agent{Name: key.agentName}
	}

	var language *modelpb.Language
	if key.serviceLanguageName != "" {
		language = &modelpb.Language{
			Name: key.serviceLanguageName,
		}
	}

	return &modelpb.APMEvent{
		Timestamp: t,
		Service: &modelpb.Service{
			Name:        key.serviceName,
			Environment: key.serviceEnvironment,
			Language:    language,
		},
		Agent:         agent,
		Labels:        key.Labels,
		NumericLabels: key.NumericLabels,
		Metricset: &modelpb.Metricset{
			DocCount: totalCount,
			Name:     metricsetName,
			Interval: interval,
		},
		Transaction: &modelpb.Transaction{
			Type:            key.transactionType,
			DurationSummary: &transactionDurationSummary,
			DurationHistogram: &modelpb.Histogram{
				Counts: counts,
				Values: values,
			},
		},
		Event: event,
	}
}
