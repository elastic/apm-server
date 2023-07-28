// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package txmetrics

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
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
	minDuration time.Duration = 0
	maxDuration time.Duration = time.Hour

	// We scale transaction counts in the histogram, which only permits storing
	// integer counts, to allow for fractional transactions due to sampling.
	//
	// e.g. if the sampling rate is 0.4, then each sampled transaction has a
	// representative count of 2.5 (1/0.4). If we receive two such transactions
	// we will record a count of 5000 (2 * 2.5 * histogramCountScale). When we
	// publish metrics, we will scale down to 5 (5000 / histogramCountScale).
	histogramCountScale = 1000

	metricsetName = "transaction"

	// overflowBucketName is an identifier to denote overflow buckets
	overflowBucketName = "_other"

	// overflowLoggerRateLimit is the maximum frequency at which overflow logs
	// are logged.
	overflowLoggerRateLimit = time.Minute
)

// Aggregator aggregates transaction durations, periodically publishing histogram metrics.
type Aggregator struct {
	*baseaggregator.Aggregator
	config  AggregatorConfig
	metrics *aggregatorMetrics

	overflowLogger *logp.Logger

	mu               sync.RWMutex
	active, inactive map[time.Duration]*metrics

	histogramPool sync.Pool
}

type aggregatorMetrics struct {
	mu                      sync.RWMutex
	activeGroups            int64
	txnGroupsOverflow       int64
	perSvcTxnGroupsOverflow int64
	servicesOverflow        int64
}

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// BatchProcessor is a model.BatchProcessor for asynchronously processing metrics documents.
	BatchProcessor modelpb.BatchProcessor

	// Logger is the logger for logging histogram aggregation/publishing.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger

	// MaxTransactionGroups is the maximum number of distinct transaction
	// group metrics to store within an aggregation period. Once this number
	// of groups has been reached, any new aggregation keys will be aggregated
	// in a dedicated service group identified by `_other`.
	MaxTransactionGroups int

	// MaxTransactionGroupsPerService is the maximum number of distinct
	// transaction group per service to store within an aggregation period.
	//
	// When the limit on per service transaction group is reached the new
	// transactions will be aggregated in a dedicated transaction group
	// per service identified by `_other`.
	MaxTransactionGroupsPerService int

	// MaxServices is the maximum number of distinct services that the
	// transaction groups will aggregate for.
	//
	// When the limit on service count is reached a new service, identified
	// by name `_other` will be used to aggregate all transactions within a
	// single bucket.
	MaxServices int

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
	if config.MaxTransactionGroupsPerService <= 0 {
		return errors.New("MaxTransactionGroupsPerService unspecified or negative")
	}
	if config.MaxServices <= 0 {
		return errors.New("MaxServices unspecified or negative")
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
		config:         config,
		metrics:        &aggregatorMetrics{},
		overflowLogger: config.Logger.WithOptions(logs.WithRateLimit(overflowLoggerRateLimit)),
		active:         make(map[time.Duration]*metrics),
		inactive:       make(map[time.Duration]*metrics),
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
		Interval:        config.MetricsInterval,
		RollUpIntervals: config.RollUpIntervals,
	})
	if err != nil {
		return nil, err
	}
	aggregator.Aggregator = base
	for _, interval := range aggregator.Intervals {
		aggregator.active[interval] = newMetrics(config.MaxTransactionGroups, config.MaxServices)
		aggregator.inactive[interval] = newMetrics(config.MaxTransactionGroups, config.MaxServices)
	}
	return &aggregator, nil
}

// CollectMonitoring may be called to collect monitoring metrics from the
// aggregation. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.aggregation.txmetrics" registry.
func (a *Aggregator) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	metrics := a.metrics

	a.metrics.mu.RLock()
	defer a.metrics.mu.RUnlock()

	totalOverflow := metrics.servicesOverflow + metrics.perSvcTxnGroupsOverflow + metrics.txnGroupsOverflow
	monitoring.ReportInt(V, "active_groups", metrics.activeGroups)
	monitoring.ReportNamespace(V, "overflowed", func() {
		monitoring.ReportInt(V, "services", metrics.servicesOverflow)
		monitoring.ReportInt(V, "per_service_txn_groups", metrics.perSvcTxnGroupsOverflow)
		monitoring.ReportInt(V, "txn_groups", metrics.txnGroupsOverflow)
		monitoring.ReportInt(V, "total", totalOverflow)
	})
}

func (a *Aggregator) publish(ctx context.Context, period time.Duration) error {
	// We hold a.mu only long enough to swap the metrics. This will
	// be blocked by metrics updates, which is OK, as we prefer not
	// to block metrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()
	current := a.active[period]
	a.active[period], a.inactive[period] = a.inactive[period], current
	a.mu.Unlock()

	if current.entries == 0 {
		// The condition will hold even for overflow since an overflow cannot
		// happen without adding some transaction groups.
		a.config.Logger.Debugf("no metrics to publish")
		return nil
	}

	var servicesOverflow, perSvcTxnGroupsOverflow, txnGroupsOverflow int64
	isMetricsPeriod := period == a.config.MetricsInterval
	intervalStr := interval.FormatDuration(period)
	totalActiveGroups := current.entries + current.overflowedEntries
	batch := make(modelpb.Batch, 0, totalActiveGroups)
	for svc, svcEntry := range current.m {
		for _, entries := range svcEntry.m {
			for _, entry := range entries {
				// Record the metricset interval as metricset.interval.
				event := makeMetricset(entry.transactionAggregationKey, entry.transactionMetrics, intervalStr)
				batch = append(batch, event)
				entry.reset(&a.histogramPool)
			}
		}
		if svcEntry.other != nil {
			overflowCount := int64(svcEntry.otherCardinalityEstimator.Estimate())
			if isMetricsPeriod {
				if svc == overflowBucketName {
					servicesOverflow += overflowCount
				} else if svcEntry.entries >= a.config.MaxTransactionGroupsPerService {
					perSvcTxnGroupsOverflow += overflowCount
				} else {
					txnGroupsOverflow += overflowCount
				}
			}
			entry := svcEntry.other
			// Record the metricset interval as metricset.interval.
			m := makeMetricset(entry.transactionAggregationKey, entry.transactionMetrics, intervalStr)
			m.Metricset.Samples = append(m.Metricset.Samples, &modelpb.MetricsetSample{
				Name:  "transaction.aggregation.overflow_count",
				Value: float64(overflowCount),
			})
			batch = append(batch, m)
			entry.reset(&a.histogramPool)
		}
		svcEntry.reset()
	}
	if isMetricsPeriod {
		a.metrics.mu.Lock()
		a.metrics.activeGroups += int64(totalActiveGroups)
		a.metrics.servicesOverflow += servicesOverflow
		a.metrics.perSvcTxnGroupsOverflow += perSvcTxnGroupsOverflow
		a.metrics.txnGroupsOverflow += txnGroupsOverflow
		a.metrics.mu.Unlock()
	}
	current.reset()

	a.config.Logger.Debugf("%s interval: publishing %d metricsets", period, len(batch))
	return a.config.BatchProcessor.ProcessBatch(ctx, &batch)
}

// ProcessBatch aggregates all transactions contained in "b". On overflow
// the metrics are aggregated into `_other` buckets to contain cardinality.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	for _, event := range *b {
		if event.Type() != modelpb.TransactionEventType {
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
//     aggregated under a dedicated bucket with `transaction.name` as `_other`.
//   - MaxTransactionGroups: Limits the  maximum number of transaction groups
//     that the aggregator can produce in one aggregation interval. Once the
//     limit is breached the new transactions are aggregated in the `_other`
//     transaction bucket of their corresponding services.
//   - MaxServices: Limits the maximum number of services that the aggregator
//     can aggregate over. Once this limit is breached the metrics will be
//     aggregated in the `_other` transaction bucket of a dedicated service
//     with `service.name` as `_other`.
func (a *Aggregator) AggregateTransaction(event *modelpb.APMEvent) {
	count := event.Transaction.RepresentativeCount
	if count <= 0 {
		return
	}
	for _, interval := range a.Intervals {
		key := makeTransactionAggregationKey(event, interval)
		if err := a.updateTransactionMetrics(key, count, event.GetEvent().GetDuration().AsDuration(), interval); err != nil {
			a.config.Logger.Errorf("failed to aggregate transaction: %w", err)
		}
	}
}

func (a *Aggregator) updateTransactionMetrics(key transactionAggregationKey, count float64, duration, interval time.Duration) error {
	// Ensure duration is within HDR histogram's bounds
	if duration < minDuration {
		duration = minDuration
	} else if duration > maxDuration {
		duration = maxDuration
	}

	hash := key.hash()
	// Get a read lock over the aggregator to ensure that the active metrics is not
	// published when updater is processing.
	a.mu.RLock()
	defer a.mu.RUnlock()
	m := a.active[interval]

	// Get a read lock over the metrics to search for a pre-existing transaction group
	// entry.
	m.mu.RLock()
	_, entry, offset := m.searchMetricsEntry(hash, key, 0)
	m.mu.RUnlock()
	if entry != nil {
		entry.recordDuration(duration, count)
		return nil
	}

	// If cannot find a pre-existing transaction group then acquire a write lock in
	// order to create a new transaction group. To protect against race conditions due
	// to manual upgrade of lock search for the transaction group again.
	var svc *svcMetricsMapEntry
	m.mu.Lock()
	defer m.mu.Unlock()
	svc, entry, _ = m.searchMetricsEntry(hash, key, offset)
	if entry != nil {
		entry.recordDuration(duration, count)
		return nil
	}

	// If all attempts to search for existing transaction group have failed then a new
	// transaction group is created as per the following criteria:
	// 1. If the service exists:
	//    1.a. If the number of transaction groups for the service is less than the
	//         limit for max per service transaction group, then create a new group.
	//    1.b. If the max per service transaction group limit is breached then record
	//         the metrics in the transaction group overflow bucket for the service.
	// 2. If the service doesn't exist:
	//    2.a. If the number of services is less than max services, then create a new
	//         service and record data as per criteria 1.
	//    2.b. If the max_services limit is breached, then record the metrics in the
	//         service overflow bucket.
	var err error
	var svcOverflow, perSvcTxnOverflow, txnOverflow bool
	if svc == nil {
		// consider service overflow only if a new service needs to be created.
		svcOverflow = m.services >= a.config.MaxServices
		svc, err = m.newServiceEntry(key.serviceName, svcOverflow)
		if err != nil {
			return err
		}
	}
	perSvcTxnOverflow = svc.entries >= a.config.MaxTransactionGroupsPerService
	txnOverflow = m.entries >= a.config.MaxTransactionGroups

	// If metrics are overflowing then make sure to update the cardinality estimator
	// as well as overwrite the key with the overflow key.
	overflow := svcOverflow || perSvcTxnOverflow || txnOverflow
	if overflow {
		// For `_other` service we only account for `_other` transaction bucket.
		key = makeOverflowAggregationKey(key, svcOverflow, interval)
		a.logOverflow(key.serviceName, interval, svcOverflow, perSvcTxnOverflow, txnOverflow)
	}
	entry, err = m.newMetricsEntry(svc, hash, key, &a.histogramPool, overflow)
	if err != nil {
		return err
	}
	entry.recordDuration(duration, count)
	return nil
}

func (a *Aggregator) logOverflow(svcName string, interval time.Duration, svcOverflow, perSvcTxnOverflow, txnOverflow bool) {
	if svcOverflow {
		a.overflowLogger.Warnf(`
%s Service limit of %d reached, new metric documents will be grouped under a dedicated
overflow bucket identified by service name '%s'.`[1:],
			interval.String(),
			a.config.MaxServices,
			overflowBucketName,
		)
	}
	if perSvcTxnOverflow {
		a.overflowLogger.Warnf(`
%s Transaction group limit of %d reached for service %s, new metric documents will be grouped
under a dedicated bucket identified by transaction name '%s'. This is typically
caused by ineffective transaction grouping, e.g. by creating many unique transaction
names.
If you are using an agent with 'use_path_as_transaction_name' enabled, it may cause
high cardinality. If your agent supports the 'transaction_name_groups' option, setting
that configuration option appropriately, may lead to better results.`[1:],
			interval.String(),
			a.config.MaxTransactionGroupsPerService,
			svcName,
			overflowBucketName,
		)
	}
	a.overflowLogger.Warnf(`
%s Overall transaction group limit of %d reached, new metric documents will be grouped
under a dedicated bucket identified by transaction name '%s'. This is typically
caused by ineffective transaction grouping, e.g. by creating many unique transaction
names.
If you are using an agent with 'use_path_as_transaction_name' enabled, it may cause
high cardinality. If your agent supports the 'transaction_name_groups' option, setting
that configuration option appropriately, may lead to better results.`[1:],
		interval.String(),
		a.config.MaxTransactionGroups,
		overflowBucketName,
	)
}

func makeOverflowAggregationKey(
	oldKey transactionAggregationKey,
	svcOverflow bool,
	interval time.Duration,
) transactionAggregationKey {
	svcName := oldKey.serviceName
	if svcOverflow {
		svcName = overflowBucketName
	}
	return transactionAggregationKey{
		comparable: comparable{
			// We are using `time.Now` here to align the overflow aggregation to
			// the evaluation time rather than event time. This prevents us from
			// cases of bad timestamps when the server receives some events with
			// old timestamp and these events overflow causing the indexed event
			// to have old timestamp too.
			timestamp:       time.Now().Truncate(interval),
			transactionName: overflowBucketName,
			serviceName:     svcName,
		},
	}
}

func makeTransactionAggregationKey(event *modelpb.APMEvent, interval time.Duration) transactionAggregationKey {
	key := transactionAggregationKey{
		comparable: comparable{
			traceRoot:         event.GetParentId() == "",
			transactionName:   event.GetTransaction().GetName(),
			transactionResult: event.GetTransaction().GetResult(),
			transactionType:   event.GetTransaction().GetType(),
			eventOutcome:      event.GetEvent().GetOutcome(),

			agentName:             event.GetAgent().GetName(),
			serviceEnvironment:    event.GetService().GetEnvironment(),
			serviceName:           event.GetService().GetName(),
			serviceVersion:        event.GetService().GetVersion(),
			serviceNodeName:       event.GetService().GetNode().GetName(),
			serviceRuntimeName:    event.GetService().GetRuntime().GetName(),
			serviceRuntimeVersion: event.GetService().GetRuntime().GetVersion(),

			serviceLanguageName:    event.GetService().GetLanguage().GetName(),
			serviceLanguageVersion: event.GetService().GetLanguage().GetVersion(),

			hostHostname:      event.GetHost().GetHostname(),
			hostName:          event.GetHost().GetName(),
			hostOSPlatform:    event.GetHost().GetOs().GetPlatform(),
			containerID:       event.GetContainer().GetId(),
			kubernetesPodName: event.GetKubernetes().GetPodName(),

			cloudProvider:         event.GetCloud().GetProvider(),
			cloudRegion:           event.GetCloud().GetRegion(),
			cloudAvailabilityZone: event.GetCloud().GetAvailabilityZone(),
			cloudServiceName:      event.GetCloud().GetServiceName(),
			cloudAccountID:        event.GetCloud().GetAccountId(),
			cloudAccountName:      event.GetCloud().GetAccountName(),
			cloudProjectID:        event.GetCloud().GetProjectId(),
			cloudProjectName:      event.GetCloud().GetProjectName(),
			cloudMachineType:      event.GetCloud().GetMachineType(),

			faasID:          event.GetFaas().GetId(),
			faasTriggerType: event.GetFaas().GetTriggerType(),
			faasName:        event.GetFaas().GetName(),
			faasVersion:     event.GetFaas().GetVersion(),
		},
	}
	if event.Timestamp != nil {
		// Group metrics by time interval.
		key.comparable.timestamp = event.Timestamp.AsTime().Truncate(interval)
	}
	if event.Faas != nil {
		key.comparable.faasColdstart = nullableBoolFromPtr(event.Faas.ColdStart)
	}
	key.AggregatedGlobalLabels.Read(event)
	return key
}

// makeMetricset creates a metricset with key, metrics, and interval.
// It uses result from histogram for Transaction.DurationSummary and DocCount to avoid discrepancy and UI weirdness.
func makeMetricset(key transactionAggregationKey, metrics transactionMetrics, interval string) *modelpb.APMEvent {
	totalCount, counts, values := metrics.histogramBuckets()

	var eventSuccessCount *modelpb.SummaryMetric
	switch key.eventOutcome {
	case "success":
		eventSuccessCount = &modelpb.SummaryMetric{
			Count: totalCount,
			Sum:   float64(totalCount),
		}
	case "failure":
		eventSuccessCount = &modelpb.SummaryMetric{
			Count: totalCount,
		}
	case "unknown":
		// Keep both Count and Sum as 0.
	}

	var t *timestamppb.Timestamp
	if !key.timestamp.IsZero() {
		t = timestamppb.New(key.timestamp)
	}

	transactionDurationSummary := modelpb.SummaryMetric{
		Count: totalCount,
	}
	for i, v := range values {
		transactionDurationSummary.Sum += v * float64(counts[i])
	}

	var agent *modelpb.Agent
	if key.agentName != "" {
		agent = &modelpb.Agent{Name: key.agentName}
	}

	var container *modelpb.Container
	if key.containerID != "" {
		container = &modelpb.Container{Id: key.containerID}
	}

	var hostOs *modelpb.OS
	if key.hostOSPlatform != "" {
		hostOs = &modelpb.OS{
			Platform: key.hostOSPlatform,
		}
	}

	var language *modelpb.Language
	if key.serviceLanguageName != "" || key.serviceLanguageVersion != "" {
		language = &modelpb.Language{
			Name:    key.serviceLanguageName,
			Version: key.serviceLanguageVersion,
		}
	}

	var serviceRuntime *modelpb.Runtime
	if key.serviceRuntimeName != "" || key.serviceRuntimeVersion != "" {
		serviceRuntime = &modelpb.Runtime{
			Name:    key.serviceRuntimeName,
			Version: key.serviceRuntimeVersion,
		}
	}

	var serviceNode *modelpb.ServiceNode
	if key.serviceNodeName != "" {
		serviceNode = &modelpb.ServiceNode{
			Name: key.serviceNodeName,
		}
	}

	var cloud *modelpb.Cloud
	if key.cloudProvider != "" || key.cloudRegion != "" || key.cloudAvailabilityZone != "" || key.cloudServiceName != "" || key.cloudAccountID != "" ||
		key.cloudAccountName != "" || key.cloudMachineType != "" || key.cloudProjectID != "" || key.cloudProjectName != "" {
		cloud = &modelpb.Cloud{
			Provider:         key.cloudProvider,
			Region:           key.cloudRegion,
			AvailabilityZone: key.cloudAvailabilityZone,
			ServiceName:      key.cloudServiceName,
			AccountId:        key.cloudAccountID,
			AccountName:      key.cloudAccountName,
			MachineType:      key.cloudMachineType,
			ProjectId:        key.cloudProjectID,
			ProjectName:      key.cloudProjectName,
		}
	}

	var hostStr *modelpb.Host
	if key.hostHostname != "" || key.hostName != "" || hostOs != nil {
		hostStr = &modelpb.Host{
			Hostname: key.hostHostname,
			Name:     key.hostName,
			Os:       hostOs,
		}
	}

	var service *modelpb.Service
	if key.serviceName != "" || key.serviceVersion != "" || serviceNode != nil || key.serviceEnvironment != "" || serviceRuntime != nil || language != nil {
		service = &modelpb.Service{
			Name:        key.serviceName,
			Version:     key.serviceVersion,
			Node:        serviceNode,
			Environment: key.serviceEnvironment,
			Runtime:     serviceRuntime,
			Language:    language,
		}
	}

	var kube *modelpb.Kubernetes
	if key.kubernetesPodName != "" {
		kube = &modelpb.Kubernetes{PodName: key.kubernetesPodName}
	}

	var faas *modelpb.Faas
	if key.faasColdstart.isSet || key.faasID != "" || key.faasTriggerType != "" || key.faasName != "" || key.faasVersion != "" {
		faas = &modelpb.Faas{
			ColdStart:   key.faasColdstart.toPtr(),
			Id:          key.faasID,
			TriggerType: key.faasTriggerType,
			Name:        key.faasName,
			Version:     key.faasVersion,
		}
	}

	var eventStr *modelpb.Event
	if key.eventOutcome != "" || eventSuccessCount != nil {
		eventStr = &modelpb.Event{
			Outcome:      key.eventOutcome,
			SuccessCount: eventSuccessCount,
		}
	}

	return &modelpb.APMEvent{
		Timestamp:     t,
		Agent:         agent,
		Container:     container,
		Kubernetes:    kube,
		Service:       service,
		Cloud:         cloud,
		Host:          hostStr,
		Event:         eventStr,
		Faas:          faas,
		Labels:        key.AggregatedGlobalLabels.Labels,
		NumericLabels: key.AggregatedGlobalLabels.NumericLabels,
		Metricset: &modelpb.Metricset{
			Name:     metricsetName,
			DocCount: totalCount,
			Interval: interval,
		},
		Transaction: &modelpb.Transaction{
			Name:   key.transactionName,
			Type:   key.transactionType,
			Result: key.transactionResult,
			Root:   key.traceRoot,
			DurationHistogram: &modelpb.Histogram{
				Counts: counts,
				Values: values,
			},
			DurationSummary: &transactionDurationSummary,
		},
	}
}

type metrics struct {
	mu       sync.RWMutex
	space    *baseaggregator.Space[metricsMapEntry]
	svcSpace *baseaggregator.Space[svcMetricsMapEntry]
	m        map[string]*svcMetricsMapEntry
	// entries refer to the total number of tx groups getting aggregated excluding
	// overflow. Used to identify overflow limit breaches as they only account for
	// non-overflow groups.
	entries int
	// overflowedEntries refer to the total number of overflowed transaction groups
	// getting aggregated.
	overflowedEntries int
	// services refer to the total number of services getting aggregated excluding
	// overflow. Used to identify overflow limit breaches as they only account for
	// non-overflow services.
	services int
}

func newMetrics(maxGroups, maxServices int) *metrics {
	return &metrics{
		// Total number of entries = 1 per txn + 1 overflow per svc + 1 `_other` svc
		space: baseaggregator.NewSpace[metricsMapEntry](maxGroups + maxServices + 1),
		// keep 1 reserved entry for `_other` service
		svcSpace: baseaggregator.NewSpace[svcMetricsMapEntry](maxServices + 1),
		m:        make(map[string]*svcMetricsMapEntry),
	}
}

func (m *metrics) searchMetricsEntry(
	hash uint64,
	key transactionAggregationKey,
	offset int,
) (*svcMetricsMapEntry, *metricsMapEntry, int) {
	svc, ok := m.m[key.serviceName]
	if !ok {
		return nil, nil, offset
	}
	entries, ok := svc.m[hash]
	if !ok {
		return svc, nil, offset
	}
	for ; offset < len(entries); offset++ {
		entry := entries[offset]
		if entry.transactionAggregationKey.equal(key) {
			return svc, entry, offset + 1
		}
	}
	// return offset to indicate the next index that should be searched in future
	// to continue looking for the aggregation key. This will be useful when read
	// lock is upgraded to write lock and we need to attempt another search to
	// handle race conditions.
	return svc, nil, offset
}

// newServiceEntry creates a new entry for a given service name. If overflow is true then
// it either creates a entry for overflow or returns the previously created entry.
func (m *metrics) newServiceEntry(svcName string, overflow bool) (*svcMetricsMapEntry, error) {
	var err error
	if overflow {
		// put the overflow bucket as a service map to avoid handling it separately
		// while publishing the metrics.
		svc, ok := m.m[overflowBucketName]
		if !ok {
			svc, err = m.svcSpace.Next()
			if err != nil {
				return nil, err
			}
			m.m[overflowBucketName] = svc
		}
		return svc, nil
	}

	svc, err := m.svcSpace.Next()
	if err != nil {
		return nil, err
	}
	if svc.m == nil {
		svc.m = make(map[uint64][]*metricsMapEntry)
	}
	m.m[svcName] = svc
	m.services++
	return svc, nil
}

// newMetricsEntry creates a new entry for a transaction group metric. If overflow is true then
// it either creates a entry for overflow or returns the previously created entry.
func (m *metrics) newMetricsEntry(
	svc *svcMetricsMapEntry,
	hash uint64,
	key transactionAggregationKey,
	histogramPool *sync.Pool,
	overflow bool,
) (*metricsMapEntry, error) {
	getNewMetricsMapEntry := func() (*metricsMapEntry, error) {
		entry, err := m.space.Next()
		if err != nil {
			return nil, err
		}
		entry.transactionAggregationKey = key
		entry.transactionMetrics = transactionMetrics{
			histogram: histogramPool.Get().(*hdrhistogram.Histogram),
		}
		return entry, nil
	}
	if overflow {
		if svc.other == nil {
			entry, err := getNewMetricsMapEntry()
			if err != nil {
				return nil, err
			}
			svc.other = entry
			svc.otherCardinalityEstimator = hyperloglog.New14()
			m.overflowedEntries++
		}
		svc.otherCardinalityEstimator.InsertHash(hash)
		return svc.other, nil
	}

	entry, err := getNewMetricsMapEntry()
	if err != nil {
		return entry, err
	}
	svc.m[hash] = append(svc.m[hash], entry)
	m.entries++
	svc.entries++
	return entry, nil
}

func (m *metrics) reset() {
	for k := range m.m {
		delete(m.m, k)
	}
	m.entries = 0
	m.services = 0
	m.space.Reset()
	m.svcSpace.Reset()
}

type svcMetricsMapEntry struct {
	m                         map[uint64][]*metricsMapEntry
	other                     *metricsMapEntry
	otherCardinalityEstimator *hyperloglog.Sketch
	// entries refer to the total transaction groups getting aggregated excluding overflow.
	// Used to identify overflow limit breaches as they only account for non-overflow groups.
	entries int
}

func (s *svcMetricsMapEntry) reset() {
	for k := range s.m {
		delete(s.m, k)
	}
	s.other = nil
	s.otherCardinalityEstimator = nil
	s.entries = 0
}

type metricsMapEntry struct {
	transactionMetrics
	transactionAggregationKey
}

func (e *metricsMapEntry) reset(pool *sync.Pool) {
	e.histogram.Reset()
	pool.Put(e.histogram)
	e.histogram = nil
}

type nullableBool struct {
	isSet bool
	val   bool
}

func (nb nullableBool) toPtr() *bool {
	if !nb.isSet {
		return nil
	}
	return &nb.val
}

func nullableBoolFromPtr(b *bool) nullableBool {
	return nullableBool{
		isSet: b != nil,
		val:   b != nil && *b,
	}
}

// comparable contains the fields with types which can be compared with the
// equal operator '=='.
type comparable struct {
	timestamp              time.Time
	faasColdstart          nullableBool
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

// NOTE(axw) the dimensions should be kept in sync with docs/data-model.asciidoc
// for the current documentation on the APM Server model.
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
	if k.faasColdstart.isSet && k.faasColdstart.val {
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
