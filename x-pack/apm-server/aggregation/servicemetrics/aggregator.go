// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package servicemetrics

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/model"
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
		for _, mme := range metrics {
			metricset := makeMetricset(mme.aggregationKey, mme.serviceMetrics)
			batch = append(batch, metricset)
		}
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
	if event.Transaction == nil || event.Transaction.RepresentativeCount <= 0 {
		return model.APMEvent{}
	}
	key := makeAggregationKey(event, a.config.Interval)
	metrics := makeServiceMetrics(event)
	if a.active.storeOrUpdate(key, metrics, a.config.Logger) {
		return model.APMEvent{}
	}
	return makeMetricset(key, metrics)
}

type metricsBuffer struct {
	maxSize int

	mu      sync.RWMutex
	entries int
	m       map[uint64][]*metricsMapEntry
	space   []metricsMapEntry
}

func newMetricsBuffer(maxSize int) *metricsBuffer {
	return &metricsBuffer{
		maxSize: maxSize,
		m:       make(map[uint64][]*metricsMapEntry),
		space:   make([]metricsMapEntry, maxSize),
	}
}

type metricsMapEntry struct {
	serviceMetrics
	aggregationKey
}

func (mb *metricsBuffer) storeOrUpdate(key aggregationKey, metrics serviceMetrics, logger *logp.Logger) bool {
	// hash does not use the serviceMetrics so it is safe to call concurrently.
	hash := key.hash()

	// Full lock because serviceMetrics cannot be updated atomically.
	mb.mu.Lock()
	defer mb.mu.Unlock()

	entries, ok := mb.m[hash]
	if ok {
		for offset, old := range entries {
			if old.aggregationKey.equal(key) {
				entries[offset].serviceMetrics = serviceMetrics{
					transactionDuration: old.transactionDuration + metrics.transactionDuration,
					transactionCount:    old.transactionCount + metrics.transactionCount,
					failureCount:        old.failureCount + metrics.failureCount,
					successCount:        old.successCount + metrics.successCount,
				}
				return true
			}
		}
	} else if mb.entries >= len(mb.space) {
		return false
	}

	half := mb.maxSize / 2
	if !ok {
		switch mb.entries {
		case mb.maxSize:
			return false
		case half - 1:
			logger.Warn("service metrics groups reached 50% capacity")
		case mb.maxSize - 1:
			logger.Warn("service metrics groups reached 100% capacity")
		}
	}

	entry := &mb.space[mb.entries]
	entry.aggregationKey = key

	entry.serviceMetrics = serviceMetrics{
		transactionDuration: metrics.transactionDuration,
		transactionCount:    metrics.transactionCount,
		failureCount:        metrics.failureCount,
		successCount:        metrics.successCount,
	}

	mb.m[hash] = append(entries, entry)
	mb.entries++
	return true
}

type aggregationKey struct {
	labelKeys        []string
	labels           model.Labels
	numericLabelKeys []string
	numericLabels    model.NumericLabels

	comparable
}

func (k *aggregationKey) hash() uint64 {
	var h xxhash.Digest
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(k.timestamp.UnixNano()))
	h.Write(buf[:])

	writeLabels(&h, k)
	h.WriteString(k.agentName)
	h.WriteString(k.serviceEnvironment)
	h.WriteString(k.serviceName)
	h.WriteString(k.transactionType)
	return h.Sum64()
}

func (k *aggregationKey) equal(key aggregationKey) bool {
	return k.comparable == key.comparable &&
		equalLabels(k.labels, key.labels) &&
		equalNumericLabels(k.numericLabels, key.numericLabels)
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

	for k, v := range event.Labels {
		if !v.Global {
			continue
		}
		if key.labels == nil {
			key.labels = make(model.Labels)
		}
		if len(v.Values) > 0 {
			key.labels.SetSlice(k, v.Values)
		} else {
			key.labels.Set(k, v.Value)
		}
		key.labelKeys = append(key.labelKeys, k)
	}
	for k, v := range event.NumericLabels {
		if !v.Global {
			continue
		}
		if key.numericLabels == nil {
			key.numericLabels = make(model.NumericLabels)
		}
		if len(v.Values) > 0 {
			key.numericLabels.SetSlice(k, v.Values)
		} else {
			key.numericLabels.Set(k, v.Value)
		}
		key.numericLabelKeys = append(key.numericLabelKeys, k)
	}
	sort.Strings(key.labelKeys)
	sort.Strings(key.numericLabelKeys)
	return key
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
		Labels:        key.labels,
		NumericLabels: key.numericLabels,
		Processor:     model.MetricsetProcessor,
		Metricset: &model.Metricset{
			DocCount: metricCount,
			Name:     metricsetName,
		},
		Transaction: &model.Transaction{
			Type:         key.transactionType,
			FailureCount: int(math.Round(metrics.failureCount)),
			SuccessCount: int(math.Round(metrics.successCount)),
			DurationSummary: model.SummaryMetric{
				Count: metricCount,
				Sum:   float64(time.Duration(math.Round(metrics.transactionDuration)).Microseconds()),
			},
		},
	}
}

func writeLabels(w io.Writer, aggKey *aggregationKey) {
	for _, key := range aggKey.labelKeys {
		label := aggKey.labels[key]
		io.WriteString(w, key)
		if label.Value != "" {
			io.WriteString(w, label.Value)
			continue
		}
		for _, v := range label.Values {
			io.WriteString(w, v)
		}
	}
	for _, key := range aggKey.numericLabelKeys {
		label := aggKey.numericLabels[key]
		io.WriteString(w, key)
		if label.Value != 0 {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(label.Value))
			w.Write(b[:])
			continue
		}
		for _, v := range label.Values {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
			w.Write(b[:])
		}
	}
}

// equalLabels returns true if the labels are equal. The Global property is
// ignored since only global labels are compared.
func equalLabels(l, labels model.Labels) bool {
	if len(l) != len(labels) {
		return false
	}
	for key, localV := range l {
		v, ok := labels[key]
		if !ok {
			return false
		}
		// If the slice value is set, ignore the Value field.
		if len(v.Values) == 0 && v.Value != localV.Value {
			return false
		}
		if len(v.Values) != len(localV.Values) {
			return false
		}
		for i, value := range v.Values {
			if localV.Values[i] != value {
				return false
			}
		}
	}
	return true
}

// equalNumericLabels returns true if the labels are equal. The Global property
// is ignored since only global labels are compared.
func equalNumericLabels(l, labels model.NumericLabels) bool {
	if len(l) != len(labels) {
		return false
	}
	for key, localV := range l {
		v, ok := labels[key]
		if !ok {
			return false
		}
		// If the slice value is set, ignore the Value field.
		if len(v.Values) == 0 && v.Value != localV.Value {
			return false
		}
		if len(v.Values) != len(localV.Values) {
			return false
		}
		for i, value := range v.Values {
			if localV.Values[i] != value {
				return false
			}
		}
	}
	return true
}
