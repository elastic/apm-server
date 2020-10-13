// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package txmetrics

import (
	"context"
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
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
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
)

// Aggregator aggregates transaction durations, periodically publishing histogram metrics.
type Aggregator struct {
	stopMu   sync.Mutex
	stopping chan struct{}
	stopped  chan struct{}

	config              AggregatorConfig
	metrics             aggregatorMetrics
	tooManyGroupsLogger *logp.Logger
	userAgentLookup     *userAgentLookup

	mu               sync.RWMutex
	active, inactive *metrics
}

type aggregatorMetrics struct {
	overflowed int64
}

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// Report is a publish.Reporter for reporting metrics documents.
	Report publish.Reporter

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

	// RUMUserAgentLRUSize is the size of the LRU cache for mapping RUM
	// page-load User-Agent strings to browser names.
	RUMUserAgentLRUSize int
}

// Validate validates the aggregator config.
func (config AggregatorConfig) Validate() error {
	if config.Report == nil {
		return errors.New("Report unspecified")
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
	if config.RUMUserAgentLRUSize <= 0 {
		return errors.New("RUMUserAgentLRUSize unspecified or negative")
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
	ual, err := newUserAgentLookup(config.RUMUserAgentLRUSize)
	if err != nil {
		return nil, err
	}
	return &Aggregator{
		stopping:            make(chan struct{}),
		stopped:             make(chan struct{}),
		config:              config,
		tooManyGroupsLogger: config.Logger.WithOptions(logs.WithRateLimit(tooManyGroupsLoggerRateLimit)),
		userAgentLookup:     ual,
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

	now := time.Now()
	metricsets := make([]transform.Transformable, 0, a.inactive.entries)
	for hash, entries := range a.inactive.m {
		for _, entry := range entries {
			counts, values := entry.transactionMetrics.histogramBuckets()
			metricset := makeMetricset(entry.transactionAggregationKey, hash, now, counts, values)
			metricsets = append(metricsets, &metricset)
		}
		delete(a.inactive.m, hash)
	}
	a.inactive.entries = 0

	a.config.Logger.Debugf("publishing %d metricsets", len(metricsets))
	return a.config.Report(ctx, publish.PendingReq{
		Transformables: metricsets,
		Trace:          true,
	})
}

// ProcessTransformables aggregates all transactions contained in
// "in", returning the input with any metricsets requiring immediate
// publication appended.
//
// This method is expected to be used immediately prior to publishing
// the events, so that the metricsets requiring immediate publication
// can be included in the same batch.
func (a *Aggregator) ProcessTransformables(ctx context.Context, in []transform.Transformable) ([]transform.Transformable, error) {
	out := in
	for _, tf := range in {
		if tx, ok := tf.(*model.Transaction); ok {
			if metricset := a.AggregateTransaction(tx); metricset != nil {
				out = append(out, metricset)
			}
		}
	}
	return out, nil
}

// AggregateTransaction aggregates transaction metrics.
//
// If the transaction cannot be aggregated due to the maximum number
// of transaction groups being exceeded, then a *model.Metricset will
// be returned which should be published immediately, along with the
// transaction. Otherwise, the returned metricset will be nil.
func (a *Aggregator) AggregateTransaction(tx *model.Transaction) *model.Metricset {
	if tx.RepresentativeCount <= 0 {
		return nil
	}

	key := a.makeTransactionAggregationKey(tx)
	hash := key.hash()
	count := transactionCount(tx)
	duration := time.Duration(tx.Duration * float64(time.Millisecond))
	if a.updateTransactionMetrics(key, hash, tx.RepresentativeCount, duration) {
		return nil
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
	values := []float64{float64(durationMicros(duration))}
	metricset := makeMetricset(key, hash, time.Now(), counts, values)
	return &metricset
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
			durationMicros(minDuration),
			durationMicros(maxDuration),
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

func (a *Aggregator) makeTransactionAggregationKey(tx *model.Transaction) transactionAggregationKey {
	var userAgentName string
	if tx.Type == "page-load" {
		// The APM app in Kibana has a special case for "page-load"
		// transaction types, rendering distributions by country and
		// browser. We use the same logic to decide whether or not
		// to include user_agent.name in the aggregation key.
		userAgentName = a.userAgentLookup.getUserAgentName(tx.Metadata.UserAgent.Original)
	}

	return transactionAggregationKey{
		traceRoot:          tx.ParentID == "",
		transactionName:    tx.Name,
		transactionOutcome: tx.Outcome,
		transactionResult:  tx.Result,
		transactionType:    tx.Type,

		agentName:          tx.Metadata.Service.Agent.Name,
		serviceEnvironment: tx.Metadata.Service.Environment,
		serviceName:        tx.Metadata.Service.Name,
		serviceVersion:     tx.Metadata.Service.Version,

		hostname:          tx.Metadata.System.Hostname(),
		containerID:       tx.Metadata.System.Container.ID,
		kubernetesPodName: tx.Metadata.System.Kubernetes.PodName,

		userAgentName: userAgentName,

		// TODO(axw) clientCountryISOCode, requires geoIP lookup in apm-server.
	}
}

// makeMetricset makes a Metricset from key, counts, and values, with timestamp ts.
func makeMetricset(key transactionAggregationKey, hash uint64, ts time.Time, counts []int64, values []float64) model.Metricset {
	out := model.Metricset{
		Timestamp: ts,
		Metadata: model.Metadata{
			Service: model.Service{
				Name:        key.serviceName,
				Version:     key.serviceVersion,
				Environment: key.serviceEnvironment,
				Agent:       model.Agent{Name: key.agentName},
			},
			System: model.System{
				DetectedHostname: key.hostname,
				Container:        model.Container{ID: key.containerID},
				Kubernetes:       model.Kubernetes{PodName: key.kubernetesPodName},
			},
			UserAgent: model.UserAgent{
				Name: key.userAgentName,
			},
			// TODO(axw) include client.geo.country_iso_code somewhere
		},
		Event: model.MetricsetEventCategorization{
			Outcome: key.transactionOutcome,
		},
		Transaction: model.MetricsetTransaction{
			Name:   key.transactionName,
			Type:   key.transactionType,
			Result: key.transactionResult,
			Root:   key.traceRoot,
		},
		Samples: []model.Sample{{
			Name:   "transaction.duration.histogram",
			Counts: counts,
			Values: values,
		}},
	}

	// Record an timeseries instance ID, which should be uniquely identify the aggregation key.
	var timeseriesInstanceID strings.Builder
	timeseriesInstanceID.WriteString(key.serviceName)
	timeseriesInstanceID.WriteRune(':')
	timeseriesInstanceID.WriteString(key.transactionName)
	timeseriesInstanceID.WriteRune(':')
	timeseriesInstanceID.WriteString(fmt.Sprintf("%x", hash))
	out.TimeseriesInstanceID = timeseriesInstanceID.String()

	return out
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

type transactionAggregationKey struct {
	traceRoot bool
	agentName string
	// TODO(axw) requires geoIP lookup in apm-server.
	//clientCountryISOCode string
	containerID        string
	hostname           string
	kubernetesPodName  string
	serviceEnvironment string
	serviceName        string
	serviceVersion     string
	transactionName    string
	transactionOutcome string
	transactionResult  string
	transactionType    string
	userAgentName      string
}

func (k *transactionAggregationKey) hash() uint64 {
	var h xxhash.Digest
	if k.traceRoot {
		h.WriteString("1")
	}
	h.WriteString(k.agentName)
	// TODO(axw) clientCountryISOCode
	h.WriteString(k.containerID)
	h.WriteString(k.hostname)
	h.WriteString(k.kubernetesPodName)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.serviceName)
	h.WriteString(k.serviceVersion)
	h.WriteString(k.transactionName)
	h.WriteString(k.transactionOutcome)
	h.WriteString(k.transactionResult)
	h.WriteString(k.transactionType)
	h.WriteString(k.userAgentName)
	return h.Sum64()
}

type transactionMetrics struct {
	histogram *hdrhistogram.Histogram
}

func (m *transactionMetrics) recordDuration(d time.Duration, n float64) {
	count := int64(math.Round(n * histogramCountScale))
	m.histogram.RecordValuesAtomic(durationMicros(d), count)
}

func (m *transactionMetrics) histogramBuckets() (counts []int64, values []float64) {
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
		count := math.Round(float64(b.Count) / histogramCountScale)
		counts = append(counts, int64(count))
		values = append(values, float64(b.To))
	}
	return counts, values
}

func transactionCount(tx *model.Transaction) float64 {
	if tx.RepresentativeCount > 0 {
		return tx.RepresentativeCount
	}
	return 1
}

func durationMicros(d time.Duration) int64 {
	return int64(d / time.Microsecond)
}
