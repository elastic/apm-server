// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/hdrhistogram"
	"github.com/elastic/apm-aggregation/aggregators/internal/protohash"
	"github.com/elastic/apm-aggregation/aggregators/nullable"
	"github.com/elastic/apm-data/model/modelpb"
)

const (
	spanMetricsetName    = "service_destination"
	txnMetricsetName     = "transaction"
	svcTxnMetricsetName  = "service_transaction"
	summaryMetricsetName = "service_summary"

	overflowBucketName = "_other"
)

var (
	partitionedMetricsBuilderPool sync.Pool
	eventMetricsBuilderPool       sync.Pool
)

// partitionedMetricsBuilder provides support for building partitioned
// sets of metrics from an event.
type partitionedMetricsBuilder struct {
	partitions  uint16
	serviceHash xxhash.Digest
	builders    []*eventMetricsBuilder // partitioned metrics

	// Event metrics are for exactly one service, so we create an array of a
	// single element and use that for backing the slice in CombinedMetrics.
	serviceAggregationKey    aggregationpb.ServiceAggregationKey
	serviceMetrics           aggregationpb.ServiceMetrics
	keyedServiceMetrics      aggregationpb.KeyedServiceMetrics
	keyedServiceMetricsArray [1]*aggregationpb.KeyedServiceMetrics

	// We reuse a single CombinedMetrics for all partitions, by iterating
	// through each partition's metrics and setting them on the CombinedMetrics
	// before invoking the callback in eventToCombinedMetrics.
	combinedMetrics aggregationpb.CombinedMetrics
}

func getPartitionedMetricsBuilder(
	serviceAggregationKey aggregationpb.ServiceAggregationKey,
	partitions uint16,
) *partitionedMetricsBuilder {
	p, ok := partitionedMetricsBuilderPool.Get().(*partitionedMetricsBuilder)
	if !ok {
		p = &partitionedMetricsBuilder{}
		p.keyedServiceMetrics.Key = &p.serviceAggregationKey
		p.keyedServiceMetrics.Metrics = &p.serviceMetrics
		p.keyedServiceMetricsArray[0] = &p.keyedServiceMetrics
		p.combinedMetrics.ServiceMetrics = p.keyedServiceMetricsArray[:]
	}
	p.serviceAggregationKey = serviceAggregationKey
	p.serviceHash = protohash.HashServiceAggregationKey(xxhash.Digest{}, &p.serviceAggregationKey)
	p.partitions = partitions
	return p
}

// release releases all partitioned builders back to their pools.
// Objects will be reset as needed if/when the builder is reacquired.
func (p *partitionedMetricsBuilder) release() {
	for i, mb := range p.builders {
		mb.release()
		p.builders[i] = nil
	}
	p.builders = p.builders[:0]
	partitionedMetricsBuilderPool.Put(p)
}

func (p *partitionedMetricsBuilder) processEvent(e *modelpb.APMEvent) {
	switch e.Type() {
	case modelpb.TransactionEventType:
		repCount := e.GetTransaction().GetRepresentativeCount()
		if repCount <= 0 {
			p.addServiceSummaryMetrics()
			return
		}
		duration := time.Duration(e.GetEvent().GetDuration())
		p.addTransactionMetrics(e, repCount, duration)
		p.addServiceTransactionMetrics(e, repCount, duration)
		for _, dss := range e.GetTransaction().GetDroppedSpansStats() {
			p.addDroppedSpanStatsMetrics(dss, repCount)
		}
	case modelpb.SpanEventType:
		target := e.GetService().GetTarget()
		repCount := e.GetSpan().GetRepresentativeCount()
		destSvc := e.GetSpan().GetDestinationService().GetResource()
		if repCount <= 0 || (target == nil && destSvc == "") {
			p.addServiceSummaryMetrics()
			return
		}
		p.addSpanMetrics(e, repCount)
	default:
		// All other event types should add an empty service metrics,
		// for adding to service summary metrics.
		p.addServiceSummaryMetrics()
	}
}

func (p *partitionedMetricsBuilder) addTransactionMetrics(e *modelpb.APMEvent, count float64, duration time.Duration) {
	var key aggregationpb.TransactionAggregationKey
	setTransactionKey(e, &key)
	hash := protohash.HashTransactionAggregationKey(p.serviceHash, &key)

	mb := p.get(hash)
	mb.transactionAggregationKey = key

	hdr := hdrhistogram.New()
	hdr.RecordDuration(duration, count)
	setHistogramProto(hdr, &mb.transactionHistogram)
	mb.transactionMetrics.Histogram = &mb.transactionHistogram
	mb.keyedTransactionMetricsSlice = mb.keyedTransactionMetricsArray[:]
}

func (p *partitionedMetricsBuilder) addServiceTransactionMetrics(e *modelpb.APMEvent, count float64, duration time.Duration) {
	var key aggregationpb.ServiceTransactionAggregationKey
	setServiceTransactionKey(e, &key)
	hash := protohash.HashServiceTransactionAggregationKey(p.serviceHash, &key)

	mb := p.get(hash)
	mb.serviceTransactionAggregationKey = key

	if mb.transactionMetrics.Histogram == nil {
		// mb.TransactionMetrics.Histogram will be set if the event's
		// transaction metric ended up in the same partition.
		hdr := hdrhistogram.New()
		hdr.RecordDuration(duration, count)
		setHistogramProto(hdr, &mb.transactionHistogram)
	}
	mb.serviceTransactionMetrics.Histogram = &mb.transactionHistogram
	switch e.GetEvent().GetOutcome() {
	case "failure":
		mb.serviceTransactionMetrics.SuccessCount = 0
		mb.serviceTransactionMetrics.FailureCount = count
	case "success":
		mb.serviceTransactionMetrics.SuccessCount = count
		mb.serviceTransactionMetrics.FailureCount = 0
	default:
		mb.serviceTransactionMetrics.SuccessCount = 0
		mb.serviceTransactionMetrics.FailureCount = 0
	}
	mb.keyedServiceTransactionMetricsSlice = mb.keyedServiceTransactionMetricsArray[:]
}

func (p *partitionedMetricsBuilder) addDroppedSpanStatsMetrics(dss *modelpb.DroppedSpanStats, repCount float64) {
	var key aggregationpb.SpanAggregationKey
	setDroppedSpanStatsKey(dss, &key)
	hash := protohash.HashSpanAggregationKey(p.serviceHash, &key)

	mb := p.get(hash)
	i := len(mb.keyedSpanMetricsSlice)
	if i == len(mb.keyedSpanMetricsArray) {
		// No more capacity. The spec says that when 128 dropped span
		// stats entries are reached, then any remaining entries will
		// be silently discarded.
		return
	}

	mb.spanAggregationKey[i] = key
	setDroppedSpanStatsMetrics(dss, repCount, &mb.spanMetrics[i])
	mb.keyedSpanMetrics[i].Key = &mb.spanAggregationKey[i]
	mb.keyedSpanMetrics[i].Metrics = &mb.spanMetrics[i]
	mb.keyedSpanMetricsSlice = append(mb.keyedSpanMetricsSlice, &mb.keyedSpanMetrics[i])
}

func (p *partitionedMetricsBuilder) addSpanMetrics(e *modelpb.APMEvent, repCount float64) {
	var key aggregationpb.SpanAggregationKey
	setSpanKey(e, &key)
	hash := protohash.HashSpanAggregationKey(p.serviceHash, &key)

	mb := p.get(hash)
	i := len(mb.keyedSpanMetricsSlice)
	mb.spanAggregationKey[i] = key
	setSpanMetrics(e, repCount, &mb.spanMetrics[i])
	mb.keyedSpanMetrics[i].Key = &mb.spanAggregationKey[i]
	mb.keyedSpanMetrics[i].Metrics = &mb.spanMetrics[i]
	mb.keyedSpanMetricsSlice = append(mb.keyedSpanMetricsSlice, &mb.keyedSpanMetrics[i])
}

func (p *partitionedMetricsBuilder) addServiceSummaryMetrics() {
	// There are no actual metric values, we're just want to
	// create documents for the dimensions, so we can build a
	// list of services.
	_ = p.get(p.serviceHash)
}

func (p *partitionedMetricsBuilder) get(h xxhash.Digest) *eventMetricsBuilder {
	partition := uint16(h.Sum64() % uint64(p.partitions))
	for _, mb := range p.builders {
		if mb.partition == partition {
			return mb
		}
	}
	mb := getEventMetricsBuilder(partition)
	p.builders = append(p.builders, mb)
	return mb
}

// eventMetricsBuilder holds memory for the contents of per-partition
// ServiceMetrics. Each instance of the struct is capable of holding
// as many metrics as may be produced for a single event.
//
// For each metric type, the builder holds:
//   - an array with enough capacity to hold the maximum possible
//     number of that type
//   - a slice of the array, for tracking the actual number of
//     metrics of that type that will be produced; this is what
//     will be used for setting CombinedMetrics fields
//   - for each array element, space for an aggregation key
//   - for each array element, space for the metric values
type eventMetricsBuilder struct {
	partition uint16

	// Preallocate space for a single-valued histogram. This histogram may
	// be used for either or both transaction and service transaction metrics.
	transactionHDRHistogramRepresentation *hdrhistogram.HistogramRepresentation
	transactionHistogramCounts            [1]int64
	transactionHistogramBuckets           [1]int32
	transactionHistogram                  aggregationpb.HDRHistogram

	// There can be at most 1 transaction metric per event.
	transactionAggregationKey    aggregationpb.TransactionAggregationKey
	transactionMetrics           aggregationpb.TransactionMetrics
	keyedTransactionMetrics      aggregationpb.KeyedTransactionMetrics
	keyedTransactionMetricsArray [1]*aggregationpb.KeyedTransactionMetrics
	keyedTransactionMetricsSlice []*aggregationpb.KeyedTransactionMetrics

	// There can be at most 1 service transaction metric per event.
	serviceTransactionAggregationKey    aggregationpb.ServiceTransactionAggregationKey
	serviceTransactionMetrics           aggregationpb.ServiceTransactionMetrics
	keyedServiceTransactionMetrics      aggregationpb.KeyedServiceTransactionMetrics
	keyedServiceTransactionMetricsArray [1]*aggregationpb.KeyedServiceTransactionMetrics
	keyedServiceTransactionMetricsSlice []*aggregationpb.KeyedServiceTransactionMetrics

	// There can be at most 128 span metrics per event:
	// - exactly 1 for a span event
	// - at most 128 (dropped span stats) for a transaction event (1)
	//
	// (1) https://github.com/elastic/apm/blob/main/specs/agents/handling-huge-traces/tracing-spans-dropped-stats.md#limits
	spanAggregationKey    [128]aggregationpb.SpanAggregationKey
	spanMetrics           [128]aggregationpb.SpanMetrics
	keyedSpanMetrics      [128]aggregationpb.KeyedSpanMetrics
	keyedSpanMetricsArray [128]*aggregationpb.KeyedSpanMetrics
	keyedSpanMetricsSlice []*aggregationpb.KeyedSpanMetrics
}

func getEventMetricsBuilder(partition uint16) *eventMetricsBuilder {
	mb, ok := eventMetricsBuilderPool.Get().(*eventMetricsBuilder)
	if ok {
		mb.partition = partition
		// Explicitly reset instead of invoking `Reset` to avoid extra cost due to
		// additional protobuf specfic resetting logic implemented by `Reset`.
		mb.serviceTransactionMetrics = aggregationpb.ServiceTransactionMetrics{}
		mb.transactionMetrics = aggregationpb.TransactionMetrics{}
		for i := range mb.spanMetrics {
			mb.spanMetrics[i] = aggregationpb.SpanMetrics{}
		}
		mb.transactionHDRHistogramRepresentation.CountsRep.Reset()
		mb.keyedServiceTransactionMetricsSlice = mb.keyedServiceTransactionMetricsSlice[:0]
		mb.keyedTransactionMetricsSlice = mb.keyedTransactionMetricsSlice[:0]
		mb.keyedSpanMetricsSlice = mb.keyedSpanMetricsSlice[:0]
		return mb
	}
	mb = &eventMetricsBuilder{partition: partition}
	mb.transactionHDRHistogramRepresentation = hdrhistogram.New()
	mb.transactionHistogram.Counts = mb.transactionHistogramCounts[:0]
	mb.transactionHistogram.Buckets = mb.transactionHistogramBuckets[:0]
	mb.transactionMetrics.Histogram = nil
	mb.keyedTransactionMetrics.Key = &mb.transactionAggregationKey
	mb.keyedTransactionMetrics.Metrics = &mb.transactionMetrics
	mb.keyedTransactionMetricsArray[0] = &mb.keyedTransactionMetrics
	mb.keyedTransactionMetricsSlice = mb.keyedTransactionMetricsArray[:0]
	mb.keyedServiceTransactionMetrics.Key = &mb.serviceTransactionAggregationKey
	mb.keyedServiceTransactionMetrics.Metrics = &mb.serviceTransactionMetrics
	mb.keyedServiceTransactionMetricsArray[0] = &mb.keyedServiceTransactionMetrics
	mb.keyedServiceTransactionMetricsSlice = mb.keyedServiceTransactionMetricsArray[:0]
	mb.keyedSpanMetricsSlice = mb.keyedSpanMetricsArray[:0]
	return mb
}

// release releases the builder back to the pool.
// Objects will be reset as needed if/when the builder is reacquired.
func (mb *eventMetricsBuilder) release() {
	eventMetricsBuilderPool.Put(mb)
}

// eventToCombinedMetrics converts APMEvent to one or more CombinedMetrics and
// calls the provided callback for each pair of CombinedMetricsKey and
// CombinedMetrics. The callback MUST NOT hold the reference of the passed
// CombinedMetrics. If required, the callback can call CloneVT to clone the
// CombinedMetrics. If an event results in multiple metrics, they may be spread
// across different partitions.
//
// eventToCombinedMetrics will never produce overflow metrics, as it applies to a
// single APMEvent.
func eventToCombinedMetrics(
	e *modelpb.APMEvent,
	unpartitionedKey CombinedMetricsKey,
	partitions uint16,
	callback func(CombinedMetricsKey, *aggregationpb.CombinedMetrics) error,
) error {
	globalLabels, err := marshalEventGlobalLabels(e)
	if err != nil {
		return fmt.Errorf("failed to marshal global labels: %w", err)
	}

	pmb := getPartitionedMetricsBuilder(
		aggregationpb.ServiceAggregationKey{
			Timestamp: modelpb.FromTime(
				modelpb.ToTime(e.GetTimestamp()).Truncate(unpartitionedKey.Interval),
			),
			ServiceName:         e.GetService().GetName(),
			ServiceEnvironment:  e.GetService().GetEnvironment(),
			ServiceLanguageName: e.GetService().GetLanguage().GetName(),
			AgentName:           e.GetAgent().GetName(),
			GlobalLabelsStr:     globalLabels,
		},
		partitions,
	)
	defer pmb.release()

	pmb.processEvent(e)
	if len(pmb.builders) == 0 {
		// This is unexpected state as any APMEvent must result in atleast the
		// service summary metric. If such a state happens then it would indicate
		// a bug in `processEvent`.
		return fmt.Errorf("service summary metric must be produced for any event")
	}

	// Approximate events total by uniformly distributing the events total
	// amongst the partitioned key values.
	pmb.combinedMetrics.EventsTotal = 1 / float64(len(pmb.builders))
	pmb.combinedMetrics.YoungestEventTimestamp = e.GetEvent().GetReceived()

	var errs []error
	for _, mb := range pmb.builders {
		key := unpartitionedKey
		key.PartitionID = mb.partition
		pmb.serviceMetrics.TransactionMetrics = mb.keyedTransactionMetricsSlice
		pmb.serviceMetrics.ServiceTransactionMetrics = mb.keyedServiceTransactionMetricsSlice
		pmb.serviceMetrics.SpanMetrics = mb.keyedSpanMetricsSlice
		if err := callback(key, &pmb.combinedMetrics); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed while executing callback: %w", errors.Join(errs...))
	}
	return nil
}

// CombinedMetricsToBatch converts CombinedMetrics to a batch of APMEvents.
// Events in the batch are popualted using vtproto's sync pool and should be
// released back to the pool using `APMEvent#ReturnToVTPool`.
func CombinedMetricsToBatch(
	cm *aggregationpb.CombinedMetrics,
	processingTime time.Time,
	aggInterval time.Duration,
) (*modelpb.Batch, error) {
	if cm == nil || len(cm.ServiceMetrics) == 0 {
		return nil, nil
	}

	var batchSize int

	// service_summary overflow metric
	if len(cm.OverflowServicesEstimator) > 0 {
		batchSize++
		if len(cm.OverflowServices.OverflowTransactionsEstimator) > 0 {
			batchSize++
		}
		if len(cm.OverflowServices.OverflowServiceTransactionsEstimator) > 0 {
			batchSize++
		}
		if len(cm.OverflowServices.OverflowSpansEstimator) > 0 {
			batchSize++
		}
	}

	for _, ksm := range cm.ServiceMetrics {
		sm := ksm.Metrics
		batchSize += len(sm.TransactionMetrics)
		batchSize += len(sm.ServiceTransactionMetrics)
		batchSize += len(sm.SpanMetrics)
		batchSize++ // Each service will create a service summary metric
		if sm.OverflowGroups == nil {
			continue
		}
		if len(sm.OverflowGroups.OverflowTransactionsEstimator) > 0 {
			batchSize++
		}
		if len(sm.OverflowGroups.OverflowServiceTransactionsEstimator) > 0 {
			batchSize++
		}
		if len(sm.OverflowGroups.OverflowSpansEstimator) > 0 {
			batchSize++
		}
	}

	b := make(modelpb.Batch, 0, batchSize)
	aggIntervalStr := formatDuration(aggInterval)
	now := time.Now()
	for _, ksm := range cm.ServiceMetrics {
		sk, sm := ksm.Key, ksm.Metrics

		var gl globalLabels
		if err := gl.UnmarshalBinary(sk.GlobalLabelsStr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal global labels: %w", err)
		}
		getBaseEventWithLabels := func() *modelpb.APMEvent {
			event := getBaseEvent(sk, now)
			event.Labels = gl.Labels
			event.NumericLabels = gl.NumericLabels
			return event
		}

		// transaction metrics
		for _, ktm := range sm.TransactionMetrics {
			event := getBaseEventWithLabels()
			txnMetricsToAPMEvent(ktm.Key, ktm.Metrics, event, aggIntervalStr)
			b = append(b, event)
		}
		// service transaction metrics
		for _, kstm := range sm.ServiceTransactionMetrics {
			event := getBaseEventWithLabels()
			svcTxnMetricsToAPMEvent(kstm.Key, kstm.Metrics, event, aggIntervalStr)
			b = append(b, event)
		}
		// service destination metrics
		for _, kspm := range sm.SpanMetrics {
			event := getBaseEventWithLabels()
			spanMetricsToAPMEvent(kspm.Key, kspm.Metrics, event, aggIntervalStr)
			b = append(b, event)
		}

		// service summary metrics
		event := getBaseEventWithLabels()
		serviceMetricsToAPMEvent(event, aggIntervalStr)
		b = append(b, event)

		if sm.OverflowGroups == nil {
			continue
		}
		if len(sm.OverflowGroups.OverflowTransactionsEstimator) > 0 {
			estimator := hllSketch(sm.OverflowGroups.OverflowTransactionsEstimator)
			event := getBaseEvent(sk, now)
			overflowTxnMetricsToAPMEvent(
				processingTime,
				sm.OverflowGroups.OverflowTransactions,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)
		}
		if len(sm.OverflowGroups.OverflowServiceTransactionsEstimator) > 0 {
			estimator := hllSketch(
				sm.OverflowGroups.OverflowServiceTransactionsEstimator,
			)
			event := getBaseEvent(sk, now)
			overflowSvcTxnMetricsToAPMEvent(
				processingTime,
				sm.OverflowGroups.OverflowServiceTransactions,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)
		}
		if len(sm.OverflowGroups.OverflowSpansEstimator) > 0 {
			estimator := hllSketch(sm.OverflowGroups.OverflowSpansEstimator)
			event := getBaseEvent(sk, now)
			overflowSpanMetricsToAPMEvent(
				processingTime,
				sm.OverflowGroups.OverflowSpans,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)
		}
	}
	if len(cm.OverflowServicesEstimator) > 0 {
		estimator := hllSketch(cm.OverflowServicesEstimator)
		event := getOverflowBaseEvent(cm.YoungestEventTimestamp)
		overflowServiceMetricsToAPMEvent(
			processingTime,
			estimator.Estimate(),
			event,
			aggIntervalStr,
		)
		b = append(b, event)
		if len(cm.OverflowServices.OverflowTransactionsEstimator) > 0 {
			estimator := hllSketch(cm.OverflowServices.OverflowTransactionsEstimator)
			event := getOverflowBaseEvent(cm.YoungestEventTimestamp)
			overflowTxnMetricsToAPMEvent(
				processingTime,
				cm.OverflowServices.OverflowTransactions,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)

		}
		if len(cm.OverflowServices.OverflowServiceTransactionsEstimator) > 0 {
			estimator := hllSketch(
				cm.OverflowServices.OverflowServiceTransactionsEstimator,
			)
			event := getOverflowBaseEvent(cm.YoungestEventTimestamp)
			overflowSvcTxnMetricsToAPMEvent(
				processingTime,
				cm.OverflowServices.OverflowServiceTransactions,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)
		}
		if len(cm.OverflowServices.OverflowSpansEstimator) > 0 {
			estimator := hllSketch(cm.OverflowServices.OverflowSpansEstimator)
			event := getOverflowBaseEvent(cm.YoungestEventTimestamp)
			overflowSpanMetricsToAPMEvent(
				processingTime,
				cm.OverflowServices.OverflowSpans,
				estimator.Estimate(),
				event,
				aggIntervalStr,
			)
			b = append(b, event)
		}
	}
	return &b, nil
}

func setSpanMetrics(e *modelpb.APMEvent, repCount float64, out *aggregationpb.SpanMetrics) {
	var count uint32 = 1
	duration := time.Duration(e.GetEvent().GetDuration())
	if composite := e.GetSpan().GetComposite(); composite != nil {
		count = composite.GetCount()
		duration = time.Duration(composite.GetSum() * float64(time.Millisecond))
	}
	out.Count = float64(count) * repCount
	out.Sum = float64(duration) * repCount
}

func setDroppedSpanStatsMetrics(dss *modelpb.DroppedSpanStats, repCount float64, out *aggregationpb.SpanMetrics) {
	out.Count = float64(dss.GetDuration().GetCount()) * repCount
	out.Sum = float64(dss.GetDuration().GetSum()) * repCount
}

func getBaseEvent(
	key *aggregationpb.ServiceAggregationKey,
	received time.Time,
) *modelpb.APMEvent {
	event := &modelpb.APMEvent{}
	event.Timestamp = key.Timestamp
	event.Metricset = &modelpb.Metricset{}
	event.Service = &modelpb.Service{}
	event.Service.Name = key.ServiceName
	event.Service.Environment = key.ServiceEnvironment

	if key.ServiceLanguageName != "" {
		event.Service.Language = &modelpb.Language{}
		event.Service.Language.Name = key.ServiceLanguageName
	}

	if key.AgentName != "" {
		event.Agent = &modelpb.Agent{}
		event.Agent.Name = key.AgentName
	}

	event.Event = &modelpb.Event{}
	event.Event.Received = modelpb.FromTime(received)

	return event
}

func getOverflowBaseEvent(youngestEventTS uint64) *modelpb.APMEvent {
	e := &modelpb.APMEvent{}
	e.Metricset = &modelpb.Metricset{}
	e.Service = &modelpb.Service{}
	e.Service.Name = overflowBucketName

	e.Event = &modelpb.Event{}
	e.Event.Received = youngestEventTS
	return e
}

func serviceMetricsToAPMEvent(
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	// Most service keys will already be present in the base event
	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Name = summaryMetricsetName
	baseEvent.Metricset.Interval = intervalStr
}

func txnMetricsToAPMEvent(
	key *aggregationpb.TransactionAggregationKey,
	metrics *aggregationpb.TransactionMetrics,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	histogram := hdrhistogram.New()
	histogramFromProto(histogram, metrics.Histogram)
	totalCount, counts, values := histogram.Buckets()
	eventSuccessCount := &modelpb.SummaryMetric{}
	switch key.EventOutcome {
	case "success":
		eventSuccessCount.Count = totalCount
		eventSuccessCount.Sum = float64(totalCount)
	case "failure":
		eventSuccessCount.Count = totalCount
	case "unknown":
		// Keep both Count and Sum as 0.
	}
	transactionDurationSummary := &modelpb.SummaryMetric{}
	transactionDurationSummary.Count = totalCount
	for i, v := range values {
		transactionDurationSummary.Sum += v * float64(counts[i])
	}

	if baseEvent.Transaction == nil {
		baseEvent.Transaction = &modelpb.Transaction{}
	}
	baseEvent.Transaction.Name = key.TransactionName
	baseEvent.Transaction.Type = key.TransactionType
	baseEvent.Transaction.Result = key.TransactionResult
	baseEvent.Transaction.Root = key.TraceRoot
	baseEvent.Transaction.DurationSummary = transactionDurationSummary
	baseEvent.Transaction.DurationHistogram = &modelpb.Histogram{}
	baseEvent.Transaction.DurationHistogram.Counts = counts
	baseEvent.Transaction.DurationHistogram.Values = values

	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Name = txnMetricsetName
	baseEvent.Metricset.DocCount = totalCount
	baseEvent.Metricset.Interval = intervalStr

	if baseEvent.Event == nil {
		baseEvent.Event = &modelpb.Event{}
	}
	baseEvent.Event.Outcome = key.EventOutcome
	baseEvent.Event.SuccessCount = eventSuccessCount

	if key.ContainerId != "" {
		if baseEvent.Container == nil {
			baseEvent.Container = &modelpb.Container{}
		}
		baseEvent.Container.Id = key.ContainerId
	}

	if key.KubernetesPodName != "" {
		if baseEvent.Kubernetes == nil {
			baseEvent.Kubernetes = &modelpb.Kubernetes{}
		}
		baseEvent.Kubernetes.PodName = key.KubernetesPodName
	}

	if key.ServiceVersion != "" {
		if baseEvent.Service == nil {
			baseEvent.Service = &modelpb.Service{}
		}
		baseEvent.Service.Version = key.ServiceVersion
	}

	if key.ServiceNodeName != "" {
		if baseEvent.Service == nil {
			baseEvent.Service = &modelpb.Service{}
		}
		if baseEvent.Service.Node == nil {
			baseEvent.Service.Node = &modelpb.ServiceNode{}
		}
		baseEvent.Service.Node.Name = key.ServiceNodeName
	}

	if key.ServiceRuntimeName != "" ||
		key.ServiceRuntimeVersion != "" {

		if baseEvent.Service == nil {
			baseEvent.Service = &modelpb.Service{}
		}
		if baseEvent.Service.Runtime == nil {
			baseEvent.Service.Runtime = &modelpb.Runtime{}
		}
		baseEvent.Service.Runtime.Name = key.ServiceRuntimeName
		baseEvent.Service.Runtime.Version = key.ServiceRuntimeVersion
	}

	if key.ServiceLanguageVersion != "" {
		if baseEvent.Service == nil {
			baseEvent.Service = &modelpb.Service{}
		}
		if baseEvent.Service.Language == nil {
			baseEvent.Service.Language = &modelpb.Language{}
		}
		baseEvent.Service.Language.Version = key.ServiceLanguageVersion
	}

	if key.HostHostname != "" ||
		key.HostName != "" {

		if baseEvent.Host == nil {
			baseEvent.Host = &modelpb.Host{}
		}
		baseEvent.Host.Hostname = key.HostHostname
		baseEvent.Host.Name = key.HostName
	}

	if key.HostOsPlatform != "" {
		if baseEvent.Host == nil {
			baseEvent.Host = &modelpb.Host{}
		}
		if baseEvent.Host.Os == nil {
			baseEvent.Host.Os = &modelpb.OS{}
		}
		baseEvent.Host.Os.Platform = key.HostOsPlatform
	}

	faasColdstart := nullable.Bool(key.FaasColdstart)
	if faasColdstart != nullable.Nil ||
		key.FaasId != "" ||
		key.FaasName != "" ||
		key.FaasVersion != "" ||
		key.FaasTriggerType != "" {

		if baseEvent.Faas == nil {
			baseEvent.Faas = &modelpb.Faas{}
		}
		baseEvent.Faas.ColdStart = faasColdstart.ToBoolPtr()
		baseEvent.Faas.Id = key.FaasId
		baseEvent.Faas.Name = key.FaasName
		baseEvent.Faas.Version = key.FaasVersion
		baseEvent.Faas.TriggerType = key.FaasTriggerType
	}

	if key.CloudProvider != "" ||
		key.CloudRegion != "" ||
		key.CloudAvailabilityZone != "" ||
		key.CloudServiceName != "" ||
		key.CloudAccountId != "" ||
		key.CloudAccountName != "" ||
		key.CloudMachineType != "" ||
		key.CloudProjectId != "" ||
		key.CloudProjectName != "" {

		if baseEvent.Cloud == nil {
			baseEvent.Cloud = &modelpb.Cloud{}
		}
		baseEvent.Cloud.Provider = key.CloudProvider
		baseEvent.Cloud.Region = key.CloudRegion
		baseEvent.Cloud.AvailabilityZone = key.CloudAvailabilityZone
		baseEvent.Cloud.ServiceName = key.CloudServiceName
		baseEvent.Cloud.AccountId = key.CloudAccountId
		baseEvent.Cloud.AccountName = key.CloudAccountName
		baseEvent.Cloud.MachineType = key.CloudMachineType
		baseEvent.Cloud.ProjectId = key.CloudProjectId
		baseEvent.Cloud.ProjectName = key.CloudProjectName
	}
}

func svcTxnMetricsToAPMEvent(
	key *aggregationpb.ServiceTransactionAggregationKey,
	metrics *aggregationpb.ServiceTransactionMetrics,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	histogram := hdrhistogram.New()
	histogramFromProto(histogram, metrics.Histogram)
	totalCount, counts, values := histogram.Buckets()
	transactionDurationSummary := modelpb.SummaryMetric{
		Count: totalCount,
	}
	for i, v := range values {
		transactionDurationSummary.Sum += v * float64(counts[i])
	}

	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Name = svcTxnMetricsetName
	baseEvent.Metricset.DocCount = totalCount
	baseEvent.Metricset.Interval = intervalStr

	if baseEvent.Transaction == nil {
		baseEvent.Transaction = &modelpb.Transaction{}
	}
	baseEvent.Transaction.Type = key.TransactionType
	baseEvent.Transaction.DurationSummary = &transactionDurationSummary
	if baseEvent.Transaction.DurationHistogram == nil {
		baseEvent.Transaction.DurationHistogram = &modelpb.Histogram{}
	}
	baseEvent.Transaction.DurationHistogram.Counts = counts
	baseEvent.Transaction.DurationHistogram.Values = values

	if baseEvent.Event == nil {
		baseEvent.Event = &modelpb.Event{}
	}
	if baseEvent.Event.SuccessCount == nil {
		baseEvent.Event.SuccessCount = &modelpb.SummaryMetric{}
	}
	baseEvent.Event.SuccessCount.Count =
		uint64(math.Round(metrics.SuccessCount + metrics.FailureCount))
	baseEvent.Event.SuccessCount.Sum = math.Round(metrics.SuccessCount)
}

func spanMetricsToAPMEvent(
	key *aggregationpb.SpanAggregationKey,
	metrics *aggregationpb.SpanMetrics,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	var target *modelpb.ServiceTarget
	if key.TargetName != "" || key.TargetType != "" {
		target = &modelpb.ServiceTarget{}
		target.Type = key.TargetType
		target.Name = key.TargetName
	}
	if baseEvent.Service == nil {
		baseEvent.Service = &modelpb.Service{}
	}
	baseEvent.Service.Target = target

	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Name = spanMetricsetName
	baseEvent.Metricset.DocCount = uint64(math.Round(metrics.Count))
	baseEvent.Metricset.Interval = intervalStr

	if baseEvent.Span == nil {
		baseEvent.Span = &modelpb.Span{}
	}
	baseEvent.Span.Name = key.SpanName

	if baseEvent.Span.DestinationService == nil {
		baseEvent.Span.DestinationService = &modelpb.DestinationService{}
	}
	baseEvent.Span.DestinationService.Resource = key.Resource
	if baseEvent.Span.DestinationService.ResponseTime == nil {
		baseEvent.Span.DestinationService.ResponseTime =
			&modelpb.AggregatedDuration{}
	}
	baseEvent.Span.DestinationService.ResponseTime.Count =
		uint64(math.Round(metrics.Count))
	baseEvent.Span.DestinationService.ResponseTime.Sum =
		uint64(math.Round(metrics.Sum))

	if key.Outcome != "" {
		if baseEvent.Event == nil {
			baseEvent.Event = &modelpb.Event{}
		}
		baseEvent.Event.Outcome = key.Outcome
	}
}

func overflowServiceMetricsToAPMEvent(
	processingTime time.Time,
	overflowCount uint64,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	// Overflow metrics use the processing time as their timestamp rather than
	// the event time. This makes sure that they can be associated with the
	// appropriate time when the event volume caused them to overflow.
	baseEvent.Timestamp = modelpb.FromTime(processingTime)
	serviceMetricsToAPMEvent(baseEvent, intervalStr)

	sample := &modelpb.MetricsetSample{}
	sample.Name = "service_summary.aggregation.overflow_count"
	sample.Value = float64(overflowCount)
	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Samples = append(baseEvent.Metricset.Samples, sample)
}

// overflowTxnMetricsToAPMEvent maps the fields of overflow
// transaction to the passed APMEvent. This only updates transcation
// metrics related fields and expects that service related fields
// are present in the passed APMEvent.
//
// For the doc count, unlike the span metrics which uses estimated
// overflow count, the transaction metrics uses the value derived
// from the histogram to avoid consistency issues between the
// overflow estimate and the histogram.
func overflowTxnMetricsToAPMEvent(
	processingTime time.Time,
	overflowTxn *aggregationpb.TransactionMetrics,
	overflowCount uint64,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	// Overflow metrics use the processing time as their timestamp rather than
	// the event time. This makes sure that they can be associated with the
	// appropriate time when the event volume caused them to overflow.
	baseEvent.Timestamp = modelpb.FromTime(processingTime)
	overflowKey := &aggregationpb.TransactionAggregationKey{
		TransactionName: overflowBucketName,
	}
	txnMetricsToAPMEvent(overflowKey, overflowTxn, baseEvent, intervalStr)

	sample := &modelpb.MetricsetSample{}
	sample.Name = "transaction.aggregation.overflow_count"
	sample.Value = float64(overflowCount)
	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Samples = append(baseEvent.Metricset.Samples, sample)
}

func overflowSvcTxnMetricsToAPMEvent(
	processingTime time.Time,
	overflowSvcTxn *aggregationpb.ServiceTransactionMetrics,
	overflowCount uint64,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	// Overflow metrics use the processing time as their timestamp rather than
	// the event time. This makes sure that they can be associated with the
	// appropriate time when the event volume caused them to overflow.
	baseEvent.Timestamp = modelpb.FromTime(processingTime)
	overflowKey := &aggregationpb.ServiceTransactionAggregationKey{
		TransactionType: overflowBucketName,
	}
	svcTxnMetricsToAPMEvent(overflowKey, overflowSvcTxn, baseEvent, intervalStr)

	sample := &modelpb.MetricsetSample{}
	sample.Name = "service_transaction.aggregation.overflow_count"
	sample.Value = float64(overflowCount)
	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Samples = append(baseEvent.Metricset.Samples, sample)
}

func overflowSpanMetricsToAPMEvent(
	processingTime time.Time,
	overflowSpan *aggregationpb.SpanMetrics,
	overflowCount uint64,
	baseEvent *modelpb.APMEvent,
	intervalStr string,
) {
	// Overflow metrics use the processing time as their timestamp rather than
	// the event time. This makes sure that they can be associated with the
	// appropriate time when the event volume caused them to overflow.
	baseEvent.Timestamp = modelpb.FromTime(processingTime)
	overflowKey := &aggregationpb.SpanAggregationKey{
		TargetName: overflowBucketName,
	}
	spanMetricsToAPMEvent(overflowKey, overflowSpan, baseEvent, intervalStr)

	sample := &modelpb.MetricsetSample{}
	sample.Name = "service_destination.aggregation.overflow_count"
	sample.Value = float64(overflowCount)
	if baseEvent.Metricset == nil {
		baseEvent.Metricset = &modelpb.Metricset{}
	}
	baseEvent.Metricset.Samples = append(baseEvent.Metricset.Samples, sample)
	baseEvent.Metricset.DocCount = overflowCount
}

func marshalEventGlobalLabels(e *modelpb.APMEvent) ([]byte, error) {
	var labelsCnt, numericLabelsCnt int
	for _, v := range e.Labels {
		if !v.Global {
			continue
		}
		labelsCnt++
	}
	for _, v := range e.NumericLabels {
		if !v.Global {
			continue
		}
		numericLabelsCnt++
	}

	if labelsCnt == 0 && numericLabelsCnt == 0 {
		return nil, nil
	}

	pb := &aggregationpb.GlobalLabels{}
	pb.Labels = slices.Grow(pb.Labels, labelsCnt)[:labelsCnt]
	pb.NumericLabels = slices.Grow(pb.NumericLabels, numericLabelsCnt)[:numericLabelsCnt]

	var i int
	// Keys must be sorted to ensure wire formats are deterministically generated and strings are directly comparable
	// i.e. Protobuf formats are equal if and only if the structs are equal
	for k, v := range e.Labels {
		if !v.Global {
			continue
		}
		if pb.Labels[i] == nil {
			pb.Labels[i] = &aggregationpb.Label{}
		}
		pb.Labels[i].Key = k
		pb.Labels[i].Value = v.Value
		pb.Labels[i].Values = slices.Grow(pb.Labels[i].Values, len(v.Values))[:len(v.Values)]
		copy(pb.Labels[i].Values, v.Values)
		i++
	}
	sort.Slice(pb.Labels, func(i, j int) bool {
		return pb.Labels[i].Key < pb.Labels[j].Key
	})

	i = 0
	for k, v := range e.NumericLabels {
		if !v.Global {
			continue
		}
		if pb.NumericLabels[i] == nil {
			pb.NumericLabels[i] = &aggregationpb.NumericLabel{}
		}
		pb.NumericLabels[i].Key = k
		pb.NumericLabels[i].Value = v.Value
		pb.NumericLabels[i].Values = slices.Grow(pb.NumericLabels[i].Values, len(v.Values))[:len(v.Values)]
		copy(pb.NumericLabels[i].Values, v.Values)
		i++
	}
	sort.Slice(pb.NumericLabels, func(i, j int) bool {
		return pb.NumericLabels[i].Key < pb.NumericLabels[j].Key
	})

	return pb.MarshalVT()
}

func setTransactionKey(e *modelpb.APMEvent, key *aggregationpb.TransactionAggregationKey) {
	var faasColdstart nullable.Bool
	faas := e.GetFaas()
	if faas != nil {
		faasColdstart.ParseBoolPtr(faas.ColdStart)
	}

	key.TraceRoot = e.GetParentId() == ""

	key.ContainerId = e.GetContainer().GetId()
	key.KubernetesPodName = e.GetKubernetes().GetPodName()

	key.ServiceVersion = e.GetService().GetVersion()
	key.ServiceNodeName = e.GetService().GetNode().GetName()

	key.ServiceRuntimeName = e.GetService().GetRuntime().GetName()
	key.ServiceRuntimeVersion = e.GetService().GetRuntime().GetVersion()
	key.ServiceLanguageVersion = e.GetService().GetLanguage().GetVersion()

	key.HostHostname = e.GetHost().GetHostname()
	key.HostName = e.GetHost().GetName()
	key.HostOsPlatform = e.GetHost().GetOs().GetPlatform()

	key.EventOutcome = e.GetEvent().GetOutcome()

	key.TransactionName = e.GetTransaction().GetName()
	key.TransactionType = e.GetTransaction().GetType()
	key.TransactionResult = e.GetTransaction().GetResult()

	key.FaasColdstart = uint32(faasColdstart)
	key.FaasId = faas.GetId()
	key.FaasName = faas.GetName()
	key.FaasVersion = faas.GetVersion()
	key.FaasTriggerType = faas.GetTriggerType()

	key.CloudProvider = e.GetCloud().GetProvider()
	key.CloudRegion = e.GetCloud().GetRegion()
	key.CloudAvailabilityZone = e.GetCloud().GetAvailabilityZone()
	key.CloudServiceName = e.GetCloud().GetServiceName()
	key.CloudAccountId = e.GetCloud().GetAccountId()
	key.CloudAccountName = e.GetCloud().GetAccountName()
	key.CloudMachineType = e.GetCloud().GetMachineType()
	key.CloudProjectId = e.GetCloud().GetProjectId()
	key.CloudProjectName = e.GetCloud().GetProjectName()
}

func setServiceTransactionKey(e *modelpb.APMEvent, key *aggregationpb.ServiceTransactionAggregationKey) {
	key.TransactionType = e.GetTransaction().GetType()
}

func setSpanKey(e *modelpb.APMEvent, key *aggregationpb.SpanAggregationKey) {
	var resource, targetType, targetName string
	target := e.GetService().GetTarget()
	if target != nil {
		targetType = target.GetType()
		targetName = target.GetName()
	}
	destSvc := e.GetSpan().GetDestinationService()
	if destSvc != nil {
		resource = destSvc.GetResource()
	}

	key.SpanName = e.GetSpan().GetName()
	key.Outcome = e.GetEvent().GetOutcome()
	key.TargetType = targetType
	key.TargetName = targetName
	key.Resource = resource
}

func setDroppedSpanStatsKey(dss *modelpb.DroppedSpanStats, key *aggregationpb.SpanAggregationKey) {
	// Dropped span statistics do not contain span name because it
	// would be too expensive to track dropped span stats per span name.
	key.Outcome = dss.GetOutcome()
	key.TargetType = dss.GetServiceTargetType()
	key.TargetName = dss.GetServiceTargetName()
	key.Resource = dss.GetDestinationServiceResource()
}

func formatDuration(d time.Duration) string {
	if duration := d.Minutes(); duration >= 1 {
		return fmt.Sprintf("%.0fm", duration)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}
