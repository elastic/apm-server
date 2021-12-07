// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package txmetrics

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"github.com/elastic/go-hdrhistogram"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
)

const (
	minDuration time.Duration = 0
	maxDuration time.Duration = time.Hour

	// We scale transaction counts in the histogram, which only permits storing
	// tnteger counts, to allow for fractional transactions due to sampling.
	//
	// e.g. if the sampling rate is 0.4, then each sampled transaction has a
	// representative count of 2.5 (1/0.4). If we receive two such transactions
	// we will record a count of 5000 (2 * 2.5 * histogramCountScale). When we
	// publish metrics, we will scale down to 5 (5000 / histogramCountScale).
	histogramCountScale = 1000

	// tooManyGroupsLoggerRateLimit is the maximum frequency at which
	// "too many groups" log messages are logged.
	tooManyGroupsLoggerRateLimit = time.Minute

	metricsetName = "transaction"
)

// Aggregator aggregates transaction durations, periodically publishing histogram metrics.
type Aggregator struct {
	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}

	config              AggregatorConfig
	metrics             *aggregatorMetrics // heap-allocated for 64-bit alignment
	tooManyGroupsLogger *logp.Logger

	mu               sync.RWMutex
	active, inactive *metrics
}

type aggregatorMetrics struct {
	overflowed int64
}

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// BatchProcessor is a model.BatchProcessor for asynchronously processing metrics documents.
	BatchProcessor model.BatchProcessor

	// Logger is the logger for logging histogram aggregation/publishing.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger

	// MaxTransactionGroups is the maximum number of distinct transaction
	// group metrics to store within an aggregation period. Once this number
	// of groups has been reached, any new aggregation keys will cause
	// individual metrics documents to be immediately published.
	MaxTransactionGroups int

	// MetricsInterval is the interval between publishing of aggregated
	// metrics. There may be additional metrics reported at arbitrary
	// times if the aggregation groups fill up.
	MetricsInterval time.Duration

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
	if config.MaxTransactionGroups <= 0 {
		return errors.New("MaxTransactionGroups unspecified or negative")
	}
	if config.MetricsInterval <= 0 {
		return errors.New("MetricsInterval unspecified or negative")
	}
	if n := config.HDRHistogramSignificantFigures; n < 1 || n > 5 {
		return errors.Errorf("HDRHistogramSignificantFigures (%d) outside range [1,5]", n)
	}
	return nil
}

// NewAggregator returns a new Aggregator with the given config.
func NewAggregator(config AggregatorConfig) (*Aggregator, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid aggregator config")
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.TransactionMetrics)
	}
	return &Aggregator{
		stopping:            make(chan struct{}),
		stopped:             make(chan struct{}),
		config:              config,
		metrics:             &aggregatorMetrics{},
		tooManyGroupsLogger: config.Logger.WithOptions(logs.WithRateLimit(tooManyGroupsLoggerRateLimit)),
		active:              newMetrics(config.MaxTransactionGroups),
		inactive:            newMetrics(config.MaxTransactionGroups),
	}, nil
}

// Run runs the Aggregator, periodically publishing and clearing aggregated
// metrics. Run returns when either a fatal error occurs, or the Aggregator's
// Stop method is invoked.
func (a *Aggregator) Run() error {
	ticker := time.NewTicker(a.config.MetricsInterval)
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
				"publishing transaction metrics failed: %s", err,
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

// CollectMonitoring may be called to collect monitoring metrics from the
// aggregation. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.aggregation.txmetrics" registry.
func (a *Aggregator) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active
	m.mu.RLock()
	defer m.mu.RUnlock()

	monitoring.ReportInt(V, "active_groups", int64(m.entries))
	monitoring.ReportInt(V, "overflowed", atomic.LoadInt64(&a.metrics.overflowed))
}

func (a *Aggregator) publish(ctx context.Context) error {
	// We hold a.mu only long enough to swap the metrics. This will
	// be blocked by metrics updates, which is OK, as we prefer not
	// to block metrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()
	a.active, a.inactive = a.inactive, a.active
	a.mu.Unlock()

	if a.inactive.entries == 0 {
		a.config.Logger.Debugf("no metrics to publish")
		return nil
	}

	// TODO(axw) record either the aggregation interval in effect, or
	// the specific time period (date_range) on the metrics documents.

	batch := make(model.Batch, 0, a.inactive.entries)
	for hash, entries := range a.inactive.m {
		for _, entry := range entries {
			totalCount, counts, values := entry.transactionMetrics.histogramBuckets()
			batch = append(batch, makeMetricset(entry.transactionAggregationKey, hash, totalCount, counts, values))
		}
		delete(a.inactive.m, hash)
	}
	a.inactive.entries = 0

	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all transactions contained in "b", adding to it any
// metricsets requiring immediate publication appended.
//
// This method is expected to be used immediately prior to publishing the
// events, so that the metricsets requiring immediate publication can be
// included in the same batch.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		if event.Processor != model.TransactionProcessor {
			continue
		}
		if metricsetEvent := a.AggregateTransaction(event); metricsetEvent.Metricset != nil {
			*b = append(*b, metricsetEvent)
		}
	}
	return nil
}

// AggregateTransaction aggregates transaction metrics.
//
// If the transaction cannot be aggregated due to the maximum number
// of transaction groups being exceeded, then a metricset APMEvent will
// be returned which should be published immediately, along with the
// transaction. Otherwise, the returned event will be the zero value.
func (a *Aggregator) AggregateTransaction(event model.APMEvent) model.APMEvent {
	if event.Transaction.RepresentativeCount <= 0 {
		return model.APMEvent{}
	}

	key := a.makeTransactionAggregationKey(event, a.config.MetricsInterval)
	hash := key.hash()
	count := transactionCount(event.Transaction)
	if a.updateTransactionMetrics(key, hash, event.Transaction.RepresentativeCount, event.Event.Duration) {
		return model.APMEvent{}
	}
	// Too many aggregation keys: could not update metrics, so immediately
	// publish a single-value metric document.
	a.tooManyGroupsLogger.Warn(`
Transaction group limit reached, falling back to sending individual metric documents.
This is typically caused by ineffective transaction grouping, e.g. by creating many
unique transaction names.`[1:],
	)
	atomic.AddInt64(&a.metrics.overflowed, 1)
	counts := []int64{int64(math.Round(count))}
	values := []float64{float64(event.Event.Duration.Microseconds())}
	return makeMetricset(key, hash, counts[0], counts, values)
}

func (a *Aggregator) updateTransactionMetrics(key transactionAggregationKey, hash uint64, count float64, duration time.Duration) bool {
	if duration < minDuration {
		duration = minDuration
	} else if duration > maxDuration {
		duration = maxDuration
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active
	m.mu.RLock()
	entries, ok := m.m[hash]
	m.mu.RUnlock()
	var offset int
	if ok {
		for offset = range entries {
			if entries[offset].transactionAggregationKey == key {
				entries[offset].recordDuration(duration, count)
				return true
			}
		}
		offset++ // where to start searching with the write lock below
	}

	m.mu.Lock()
	entries, ok = m.m[hash]
	if ok {
		for i := range entries[offset:] {
			if entries[offset+i].transactionAggregationKey == key {
				m.mu.Unlock()
				entries[offset+i].recordDuration(duration, count)
				return true
			}
		}
	} else if m.entries >= len(m.space) {
		m.mu.Unlock()
		return false
	}
	entry := &m.space[m.entries]
	entry.transactionAggregationKey = key
	if entry.transactionMetrics.histogram == nil {
		entry.transactionMetrics.histogram = hdrhistogram.New(
			minDuration.Microseconds(),
			maxDuration.Microseconds(),
			a.config.HDRHistogramSignificantFigures,
		)
	} else {
		entry.transactionMetrics.histogram.Reset()
	}
	entry.recordDuration(duration, count)
	m.m[hash] = append(entries, entry)
	m.entries++
	m.mu.Unlock()
	return true
}

func (a *Aggregator) makeTransactionAggregationKey(event model.APMEvent, interval time.Duration) transactionAggregationKey {
	return transactionAggregationKey{
		// Group metrics by time interval.
		timestamp: event.Timestamp.Truncate(interval),

		traceRoot:         event.Parent.ID == "",
		transactionName:   event.Transaction.Name,
		transactionResult: event.Transaction.Result,
		transactionType:   event.Transaction.Type,
		eventOutcome:      event.Event.Outcome,

		agentName:          event.Agent.Name,
		serviceEnvironment: event.Service.Environment,
		serviceName:        event.Service.Name,
		serviceVersion:     event.Service.Version,
		serviceNodeName:    event.Service.Node.Name,

		hostname:          event.Host.Hostname,
		containerID:       event.Container.ID,
		kubernetesPodName: event.Kubernetes.PodName,

		cloudProvider:         event.Cloud.Provider,
		cloudRegion:           event.Cloud.Region,
		cloudAvailabilityZone: event.Cloud.AvailabilityZone,
	}
}

// makeMetricset makes a metricset event from key, counts, and values, with timestamp ts.
func makeMetricset(
	key transactionAggregationKey, hash uint64, totalCount int64, counts []int64, values []float64,
) model.APMEvent {
	// Record a timeseries instance ID, which should be uniquely identify the aggregation key.
	var timeseriesInstanceID strings.Builder
	timeseriesInstanceID.WriteString(key.serviceName)
	timeseriesInstanceID.WriteRune(':')
	timeseriesInstanceID.WriteString(key.transactionName)
	timeseriesInstanceID.WriteRune(':')
	timeseriesInstanceID.WriteString(fmt.Sprintf("%x", hash))

	return model.APMEvent{
		Timestamp:  key.timestamp,
		Agent:      model.Agent{Name: key.agentName},
		Container:  model.Container{ID: key.containerID},
		Kubernetes: model.Kubernetes{PodName: key.kubernetesPodName},
		Service: model.Service{
			Name:        key.serviceName,
			Version:     key.serviceVersion,
			Node:        model.ServiceNode{Name: key.serviceNodeName},
			Environment: key.serviceEnvironment,
		},
		Cloud: model.Cloud{
			Provider:         key.cloudProvider,
			Region:           key.cloudRegion,
			AvailabilityZone: key.cloudAvailabilityZone,
		},
		Host: model.Host{
			Hostname: key.hostname,
		},
		Event: model.Event{
			Outcome: key.eventOutcome,
		},
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name:                 metricsetName,
			DocCount:             totalCount,
			TimeseriesInstanceID: timeseriesInstanceID.String(),
		},
		Transaction: &model.Transaction{
			Name:   key.transactionName,
			Type:   key.transactionType,
			Result: key.transactionResult,
			Root:   key.traceRoot,
			DurationHistogram: model.Histogram{
				Counts: counts,
				Values: values,
			},
		},
	}
}

type metrics struct {
	mu      sync.RWMutex
	entries int
	m       map[uint64][]*metricsMapEntry
	space   []metricsMapEntry
}

func newMetrics(maxGroups int) *metrics {
	return &metrics{
		m:     make(map[uint64][]*metricsMapEntry),
		space: make([]metricsMapEntry, maxGroups),
	}
}

type metricsMapEntry struct {
	transactionAggregationKey
	transactionMetrics
}

// NOTE(axw) the dimensions should be kept in sync with docs/metricset-indices.asciidoc.
type transactionAggregationKey struct {
	timestamp             time.Time
	traceRoot             bool
	agentName             string
	containerID           string
	hostname              string
	kubernetesPodName     string
	cloudProvider         string
	cloudRegion           string
	cloudAvailabilityZone string
	serviceEnvironment    string
	serviceName           string
	serviceVersion        string
	serviceNodeName       string
	transactionName       string
	transactionResult     string
	transactionType       string
	eventOutcome          string
}

func (k *transactionAggregationKey) hash() uint64 {
	var h xxhash.Digest
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(k.timestamp.UnixNano()))
	h.Write(buf[:])
	if k.traceRoot {
		h.WriteString("1")
	}
	h.WriteString(k.agentName)
	h.WriteString(k.containerID)
	h.WriteString(k.hostname)
	h.WriteString(k.kubernetesPodName)
	h.WriteString(k.cloudProvider)
	h.WriteString(k.cloudRegion)
	h.WriteString(k.cloudAvailabilityZone)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.serviceName)
	h.WriteString(k.serviceVersion)
	h.WriteString(k.serviceNodeName)
	h.WriteString(k.transactionName)
	h.WriteString(k.transactionResult)
	h.WriteString(k.transactionType)
	h.WriteString(k.eventOutcome)
	return h.Sum64()
}

type transactionMetrics struct {
	histogram *hdrhistogram.Histogram
}

func (m *transactionMetrics) recordDuration(d time.Duration, n float64) {
	count := int64(math.Round(n * histogramCountScale))
	m.histogram.RecordValuesAtomic(d.Microseconds(), count)
}

func (m *transactionMetrics) histogramBuckets() (totalCount int64, counts []int64, values []float64) {
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

func transactionCount(tx *model.Transaction) float64 {
	if tx.RepresentativeCount > 0 {
		return tx.RepresentativeCount
	}
	return 1
}
