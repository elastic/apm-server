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
	active, inactive map[time.Duration]*metrics

	intervals []time.Duration // List of all intervals.
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

	// RollUpIntervals are additional MetricsInterval for the aggregator to
	// compute and publish metrics for. Each additional interval is constrained
	// to the same rules as MetricsInterval, and will result in additional
	// memory to be allocated.
	RollUpIntervals []time.Duration

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
	for i, interval := range config.RollUpIntervals {
		if interval <= 0 {
			return errors.Errorf("RollUpIntervals[%d]: unspecified or negative", i)
		}
		if interval%config.MetricsInterval != 0 {
			return errors.Errorf("RollUpIntervals[%d]: interval must be a multiple of MetricsInterval", i)
		}
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
	aggregator := Aggregator{
		stopping:            make(chan struct{}),
		stopped:             make(chan struct{}),
		config:              config,
		metrics:             &aggregatorMetrics{},
		tooManyGroupsLogger: config.Logger.WithOptions(logs.WithRateLimit(tooManyGroupsLoggerRateLimit)),
		intervals:           append([]time.Duration{config.MetricsInterval}, config.RollUpIntervals...),
	}
	aggregator.active = make(map[time.Duration]*metrics)
	aggregator.inactive = make(map[time.Duration]*metrics)
	for _, interval := range aggregator.intervals {
		aggregator.active[interval] = newMetrics(config.MaxTransactionGroups)
		aggregator.inactive[interval] = newMetrics(config.MaxTransactionGroups)
	}
	return &aggregator, nil
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
	var ticks uint64
	for !stop {
		ticks++
		select {
		case <-a.stopping:
			stop = true
		case <-ticker.C:
		}
		// Publish the metricsets for all configured intervals.
		for _, interval := range a.intervals {
			// Publish $interval MetricSets when:
			//  - ticks * MetricsInterval % $interval == 0.
			//  - Aggregator is stopped.
			if !stop && (ticks*uint64(a.config.MetricsInterval))%uint64(interval) != 0 {
				continue
			}
			if err := a.publish(context.Background(), interval); err != nil {
				a.config.Logger.With(logp.Error(err)).Warnf(
					"publishing %s transaction metrics failed: %s",
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

// CollectMonitoring may be called to collect monitoring metrics from the
// aggregation. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.aggregation.txmetrics" registry.
func (a *Aggregator) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active[a.config.MetricsInterval]
	m.mu.RLock()
	defer m.mu.RUnlock()

	monitoring.ReportInt(V, "active_groups", int64(m.entries))
	monitoring.ReportInt(V, "overflowed", atomic.LoadInt64(&a.metrics.overflowed))
}

func (a *Aggregator) publish(ctx context.Context, interval time.Duration) error {
	// We hold a.mu only long enough to swap the metrics. This will
	// be blocked by metrics updates, which is OK, as we prefer not
	// to block metrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()
	current := a.active[interval]
	a.active[interval], a.inactive[interval] = a.inactive[interval], current
	a.mu.Unlock()

	if current.entries == 0 {
		a.config.Logger.Debugf("no metrics to publish")
		return nil
	}

	batch := make(model.Batch, 0, current.entries)
	for hash, entries := range current.m {
		for _, entry := range entries {
			totalCount, counts, values := entry.transactionMetrics.histogramBuckets()
			event := makeMetricset(entry.transactionAggregationKey, totalCount, counts, values)
			// Record the metricset interval as Event.Duration.
			event.Event.Duration = interval
			batch = append(batch, event)
		}
		delete(current.m, hash)
	}
	current.entries = 0

	a.config.Logger.Debugf("%s interval: publishing %d metricsets", interval.String(), len(batch))
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
		a.AggregateTransaction(event, b)
	}
	return nil
}

// AggregateTransaction aggregates transaction metrics.
//
// If the transaction cannot be aggregated due to the maximum number of
// transaction groups being exceeded, then a metricset APMEvent will be
// appended to the batch, which will be published immediately, along with the
// transaction.
func (a *Aggregator) AggregateTransaction(event model.APMEvent, batch *model.Batch) {
	if event.Transaction.RepresentativeCount <= 0 {
		return
	}
	for _, interval := range a.intervals {
		if overflowed := a.aggregateTransaction(event, interval); overflowed.Metricset != nil {
			*batch = append(*batch, overflowed)
		}
	}
}

func (a *Aggregator) aggregateTransaction(event model.APMEvent, interval time.Duration) model.APMEvent {
	key := a.makeTransactionAggregationKey(event, interval)
	hash := key.hash()
	count := transactionCount(event.Transaction)
	if a.updateTransactionMetrics(key, hash, event.Transaction.RepresentativeCount, event.Event.Duration, interval) {
		return model.APMEvent{}
	}
	// Too many aggregation keys: could not update metrics, so immediately
	// publish a single-value metric document.
	a.tooManyGroupsLogger.Warn(`
%s transaction group limit reached, falling back to sending individual metric documents.
This is typically caused by ineffective transaction grouping, e.g. by creating many
unique transaction names.
If you are using an agent with 'use_path_as_transaction_name' enabled, it may cause
high cardinality. If your agent supports the 'transaction_name_groups' option, setting
that configuration option appropriately, may lead to better results.`[1:], interval.String(),
	)
	atomic.AddInt64(&a.metrics.overflowed, 1)
	counts := []int64{int64(math.Round(count))}
	values := []float64{float64(event.Event.Duration.Microseconds())}
	return makeMetricset(key, counts[0], counts, values)
}

func (a *Aggregator) updateTransactionMetrics(key transactionAggregationKey, hash uint64, count float64, duration, interval time.Duration) bool {
	if duration < minDuration {
		duration = minDuration
	} else if duration > maxDuration {
		duration = maxDuration
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active[interval]
	m.mu.RLock()
	entries, ok := m.m[hash]
	m.mu.RUnlock()
	var offset int
	if ok {
		for offset = range entries {
			if entries[offset].transactionAggregationKey.equal(key) {
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
			if entries[offset+i].transactionAggregationKey.equal(key) {
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
	mu      sync.RWMutex
	m       map[uint64][]*metricsMapEntry
	space   []metricsMapEntry
	entries int
}

func newMetrics(maxGroups int) *metrics {
	return &metrics{
		m:     make(map[uint64][]*metricsMapEntry),
		space: make([]metricsMapEntry, maxGroups),
	}
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
