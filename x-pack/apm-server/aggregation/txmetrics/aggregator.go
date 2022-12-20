// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package txmetrics

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

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-hdrhistogram"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/labels"
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

	// MaxTransactionGroupsPerService is the maximum number of distinct
	// transaction group per service to store within an aggregation period.
	// This config is derived from `maxTransactionGroups`.
	//
	// When the limit on per service transaction group is reached the new
	// transactions will be aggregated in a dedicated transaction group
	// per service identified by `other`.
	MaxTransactionGroupsPerService int

	// MaxServices is the maximum number of distinct services that the
	// transaction groups will aggregate for.
	//
	// When the limit on service count is reached a new service, identified
	// by name `other` will be used to aggregate all transactions within a
	// single bucket.
	MaxServices int

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
	if config.MaxTransactionGroupsPerService <= 0 {
		return errors.New("MaxTransactionGroupsPerService unspecified or negative")
	}
	if config.MaxServices <= 0 {
		return errors.New("MaxServices unspecified or negative")
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
	config.Logger.Infof(
		"creating aggregator with %d txn groups limit, %d txn groups per service limit and %d services limit",
		config.MaxTransactionGroups,
		config.MaxTransactionGroupsPerService,
		config.MaxServices,
	)
	return &Aggregator{
		stopping:            make(chan struct{}),
		stopped:             make(chan struct{}),
		config:              config,
		metrics:             &aggregatorMetrics{},
		tooManyGroupsLogger: config.Logger.WithOptions(logs.WithRateLimit(tooManyGroupsLoggerRateLimit)),
		active:              newMetrics(config.MaxTransactionGroups, config.MaxServices),
		inactive:            newMetrics(config.MaxTransactionGroups, config.MaxServices),
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
	for svc, svcEntry := range a.inactive.m {
		for hash, entries := range svcEntry.m {
			for _, entry := range entries {
				totalCount, counts, values := entry.transactionMetrics.histogramBuckets()
				batch = append(batch, makeMetricset(entry.transactionAggregationKey, totalCount, counts, values))
				entry.histogram.Reset()
			}
			delete(svcEntry.m, hash)
		}
		if svcEntry.other != nil {
			entry := svcEntry.other
			totalCount, counts, values := entry.transactionMetrics.histogramBuckets()
			m := makeMetricset(entry.transactionAggregationKey, totalCount, counts, values)
			m.Metricset.Samples = append(m.Metricset.Samples, model.MetricsetSample{
				Name:  "overflow_count",
				Value: float64(svcEntry.otherCardinalityEstimator.Estimate()),
			})
			batch = append(batch, m)

			entry.histogram.Reset()
			svcEntry.other = nil
			svcEntry.otherCardinalityEstimator = nil
			svcEntry.entries = 0
		}
		delete(a.inactive.m, svc)
	}
	a.inactive.entries = 0
	a.inactive.services = 0

	a.config.Logger.Debugf("publishing %d metricsets", len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all transactions contained in "b". On overflow
// the metrics are aggregated into `other` buckets to contain cardinality.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *model.Batch) error {
	for _, event := range *b {
		if event.Processor != model.TransactionProcessor {
			continue
		}
		a.AggregateTransaction(event)
	}
	return nil
}

// AggregateTransaction aggregates transaction metrics.
//
// If the transaction cannot be aggregated due to the maximum number
// of transaction groups being exceeded, then a metricset APMEvent will
// be returned which should be published immediately, along with the
// transaction. Otherwise, the returned event will be the zero value.
//
// To contain the cardinality of the aggregated metrics the following
// limits are considered:
//
//   - MaxTransactionGroupsPerService: Limits the maximum number of
//     transactions that a specific service can produce in one aggregation
//     interval. Once the limit is breached the new transactions are
//     aggregated under a dedicated bucket with `transaction.name` as other.
//   - MaxTransactionGroups: Limits the  maximum number of transaction groups
//     that the aggregator can produce in one aggregation interval. Once the
//     limit is breached the new transactions are aggregated in the `other`
//     transaction bucket of their corresponding services.
//   - MaxServices: Limits the maximum number of services that the aggregator
//     can aggregate over. Once this limit is breached the metrics will be
//     aggregated in the `other` transaction bucket of a dedicated service
//     with `service.name` as other.
func (a *Aggregator) AggregateTransaction(event model.APMEvent) {
	if event.Transaction.RepresentativeCount <= 0 {
		return
	}

	key := a.makeTransactionAggregationKey(event, a.config.MetricsInterval)
	hash := key.hash()
	a.updateTransactionMetrics(key, hash, event.Transaction.RepresentativeCount, event.Event.Duration)
}

func (a *Aggregator) updateTransactionMetrics(key transactionAggregationKey, hash uint64, count float64, duration time.Duration) {
	if duration < minDuration {
		duration = minDuration
	} else if duration > maxDuration {
		duration = maxDuration
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active
	m.mu.RLock()
	var ok bool
	var entries []*metricsMapEntry
	svcEntry, svcOk := m.m[key.serviceName]
	if svcOk {
		entries, ok = svcEntry.m[hash]
	}
	m.mu.RUnlock()
	var offset int
	if ok {
		for offset = range entries {
			if entries[offset].transactionAggregationKey.equal(key) {
				entries[offset].recordDuration(duration, count)
				return
			}
		}
		offset++ // where to start searching with the write lock below
	}

	entries = nil
	m.mu.Lock()
	svcEntry, svcOk = m.m[key.serviceName]
	if svcOk {
		entries, ok := svcEntry.m[hash]
		if ok {
			for i := range entries[offset:] {
				if entries[offset+i].transactionAggregationKey.equal(key) {
					m.mu.Unlock()
					entries[offset+i].recordDuration(duration, count)
					return
				}
			}
		}
	} else {
		if m.services >= a.config.MaxServices {
			key.serviceName = "other"
			svcEntry = &m.svcSpace[len(m.svcSpace)-1]
		} else {
			svcEntry = &m.svcSpace[m.services]
		}
		if svcEntry.m == nil {
			svcEntry.m = make(map[uint64][]*metricsMapEntry)
		}
		m.m[key.serviceName] = svcEntry
		m.services++
	}
	var entry *metricsMapEntry
	if m.entries >= a.config.MaxTransactionGroups ||
		svcEntry.entries >= a.config.MaxTransactionGroupsPerService ||
		key.serviceName == "other" {

		if svcEntry.other != nil {
			if svcEntry.otherCardinalityEstimator.Insert([]byte(key.transactionName)) {
				atomic.AddInt64(&a.metrics.overflowed, 1)
			}
			svcEntry.other.recordDuration(duration, count)
			m.mu.Unlock()
			return
		}
		svcEntry.other = &m.space[m.entries]
		svcEntry.otherCardinalityEstimator = hyperloglog.New14()
		svcEntry.otherCardinalityEstimator.Insert([]byte(key.transactionName))
		atomic.AddInt64(&a.metrics.overflowed, 1)
		entry = svcEntry.other
		// Override transaction name with `other`. For `other` service
		// we will only account for `other` transaction bucket.
		key.transactionName = "other"
		key = a.makeOverflowAggregationKey(key)
		hash = key.hash()
	} else {
		entry = &m.space[m.entries]
		svcEntry.m[hash] = append(entries, entry)
		svcEntry.entries++
	}
	entry.transactionAggregationKey = key
	if entry.transactionMetrics.histogram == nil {
		entry.transactionMetrics.histogram = hdrhistogram.New(
			minDuration.Microseconds(),
			maxDuration.Microseconds(),
			a.config.HDRHistogramSignificantFigures,
		)
	}
	entry.recordDuration(duration, count)
	m.entries++
	m.mu.Unlock()
}

func (a *Aggregator) makeOverflowAggregationKey(oldKey transactionAggregationKey) transactionAggregationKey {
	return transactionAggregationKey{
		comparable: comparable{
			timestamp:       oldKey.timestamp,
			transactionName: oldKey.transactionName,
			serviceName:     oldKey.serviceName,
		},
		AggregatedGlobalLabels: oldKey.AggregatedGlobalLabels,
	}
}

func (a *Aggregator) makeTransactionAggregationKey(event model.APMEvent, interval time.Duration) transactionAggregationKey {
	key := transactionAggregationKey{
		comparable: comparable{
			// Group metrics by time interval.
			timestamp: event.Timestamp.Truncate(interval),

			traceRoot:         event.Parent.ID == "",
			transactionName:   event.Transaction.Name,
			transactionResult: event.Transaction.Result,
			transactionType:   event.Transaction.Type,
			eventOutcome:      event.Event.Outcome,

			agentName:             event.Agent.Name,
			serviceEnvironment:    event.Service.Environment,
			serviceName:           event.Service.Name,
			serviceVersion:        event.Service.Version,
			serviceNodeName:       event.Service.Node.Name,
			serviceRuntimeName:    event.Service.Runtime.Name,
			serviceRuntimeVersion: event.Service.Runtime.Version,

			serviceLanguageName:    event.Service.Language.Name,
			serviceLanguageVersion: event.Service.Language.Version,

			hostHostname:      event.Host.Hostname,
			hostName:          event.Host.Name,
			hostOSPlatform:    event.Host.OS.Platform,
			containerID:       event.Container.ID,
			kubernetesPodName: event.Kubernetes.PodName,

			cloudProvider:         event.Cloud.Provider,
			cloudRegion:           event.Cloud.Region,
			cloudAvailabilityZone: event.Cloud.AvailabilityZone,
			cloudServiceName:      event.Cloud.ServiceName,
			cloudAccountID:        event.Cloud.AccountID,
			cloudAccountName:      event.Cloud.AccountName,
			cloudProjectID:        event.Cloud.ProjectID,
			cloudProjectName:      event.Cloud.ProjectName,
			cloudMachineType:      event.Cloud.MachineType,

			faasColdstart:   event.FAAS.Coldstart,
			faasID:          event.FAAS.ID,
			faasTriggerType: event.FAAS.TriggerType,
			faasName:        event.FAAS.Name,
			faasVersion:     event.FAAS.Version,
		},
	}
	key.AggregatedGlobalLabels.Read(&event)
	return key
}

// makeMetricset makes a metricset event from key, counts, and values, with timestamp ts.
func makeMetricset(key transactionAggregationKey, totalCount int64, counts []int64, values []float64) model.APMEvent {
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
			Runtime: model.Runtime{
				Name:    key.serviceRuntimeName,
				Version: key.serviceRuntimeVersion,
			},
			Language: model.Language{
				Name:    key.serviceLanguageName,
				Version: key.serviceLanguageVersion,
			},
		},
		Cloud: model.Cloud{
			Provider:         key.cloudProvider,
			Region:           key.cloudRegion,
			AvailabilityZone: key.cloudAvailabilityZone,
			ServiceName:      key.cloudServiceName,
			AccountID:        key.cloudAccountID,
			AccountName:      key.cloudAccountName,
			MachineType:      key.cloudMachineType,
			ProjectID:        key.cloudProjectID,
			ProjectName:      key.cloudProjectName,
		},
		Host: model.Host{
			Hostname: key.hostHostname,
			Name:     key.hostName,
			OS: model.OS{
				Platform: key.hostOSPlatform,
			},
		},
		Event: model.Event{
			Outcome: key.eventOutcome,
		},
		FAAS: model.FAAS{
			Coldstart:   key.faasColdstart,
			ID:          key.faasID,
			TriggerType: key.faasTriggerType,
			Name:        key.faasName,
			Version:     key.faasVersion,
		},
		Labels:        key.AggregatedGlobalLabels.Labels,
		NumericLabels: key.AggregatedGlobalLabels.NumericLabels,
		Processor:     model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Name:     metricsetName,
			DocCount: totalCount,
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
	mu       sync.RWMutex
	entries  int // total number of tx groups getting aggregated
	services int
	space    []metricsMapEntry
	svcSpace []svcMetricsMapEntry
	m        map[string]*svcMetricsMapEntry
}

func newMetrics(maxGroups, maxServices int) *metrics {
	return &metrics{
		space:    make([]metricsMapEntry, maxGroups*2+1),
		svcSpace: make([]svcMetricsMapEntry, maxServices+1),
		m:        make(map[string]*svcMetricsMapEntry),
	}
}

type svcMetricsMapEntry struct {
	entries                   int // total number of tx groups for this svc getting aggregated
	m                         map[uint64][]*metricsMapEntry
	other                     *metricsMapEntry
	otherCardinalityEstimator *hyperloglog.Sketch
}

type metricsMapEntry struct {
	transactionMetrics
	transactionAggregationKey
}

// comparable contains the fields with types which can be compared with the
// equal operator '=='.
type comparable struct {
	timestamp              time.Time
	faasColdstart          *bool
	faasID                 string
	faasName               string
	faasVersion            string
	agentName              string
	hostOSPlatform         string
	kubernetesPodName      string
	cloudProvider          string
	cloudRegion            string
	cloudAvailabilityZone  string
	cloudServiceName       string
	cloudAccountID         string
	cloudAccountName       string
	cloudMachineType       string
	cloudProjectID         string
	cloudProjectName       string
	serviceEnvironment     string
	serviceName            string
	serviceVersion         string
	serviceNodeName        string
	serviceRuntimeName     string
	serviceRuntimeVersion  string
	serviceLanguageName    string
	serviceLanguageVersion string
	transactionName        string
	transactionResult      string
	transactionType        string
	eventOutcome           string
	faasTriggerType        string
	hostHostname           string
	hostName               string
	containerID            string
	traceRoot              bool
}

// NOTE(axw) the dimensions should be kept in sync with docs/metricset-indices.asciidoc (legacy).
// And docs/data-model.asciidoc for the current documentation on the APM Server model.
type transactionAggregationKey struct {
	labels.AggregatedGlobalLabels
	comparable
}

func (k *transactionAggregationKey) hash() uint64 {
	var h xxhash.Digest
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(k.timestamp.UnixNano()))
	h.Write(buf[:])
	if k.traceRoot {
		h.WriteString("1")
	}
	if k.faasColdstart != nil && *k.faasColdstart {
		h.WriteString("1")
	}
	k.AggregatedGlobalLabels.Write(&h)
	h.WriteString(k.agentName)
	h.WriteString(k.containerID)
	h.WriteString(k.hostHostname)
	h.WriteString(k.hostName)
	h.WriteString(k.hostOSPlatform)
	h.WriteString(k.kubernetesPodName)
	h.WriteString(k.cloudProvider)
	h.WriteString(k.cloudRegion)
	h.WriteString(k.cloudAvailabilityZone)
	h.WriteString(k.cloudServiceName)
	h.WriteString(k.cloudAccountID)
	h.WriteString(k.cloudAccountName)
	h.WriteString(k.cloudMachineType)
	h.WriteString(k.cloudProjectID)
	h.WriteString(k.cloudProjectName)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.serviceName)
	h.WriteString(k.serviceVersion)
	h.WriteString(k.serviceNodeName)
	h.WriteString(k.serviceRuntimeName)
	h.WriteString(k.serviceRuntimeVersion)
	h.WriteString(k.serviceLanguageName)
	h.WriteString(k.serviceLanguageVersion)
	h.WriteString(k.transactionName)
	h.WriteString(k.transactionResult)
	h.WriteString(k.transactionType)
	h.WriteString(k.eventOutcome)
	h.WriteString(k.faasID)
	h.WriteString(k.faasTriggerType)
	h.WriteString(k.faasName)
	h.WriteString(k.faasVersion)
	return h.Sum64()
}

func (k *transactionAggregationKey) equal(key transactionAggregationKey) bool {
	return k.comparable == key.comparable &&
		k.AggregatedGlobalLabels.Equals(&key.AggregatedGlobalLabels)
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
