// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"io"
	"slices"
	"sort"

	"github.com/cespare/xxhash/v2"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/constraint"
	"github.com/elastic/apm-aggregation/aggregators/internal/protohash"
)

type combinedMetricsMerger struct {
	limits      Limits
	constraints constraints
	metrics     combinedMetrics
}

func (m *combinedMetricsMerger) MergeNewer(value []byte) error {
	from := &aggregationpb.CombinedMetrics{}
	if err := from.UnmarshalVT(value); err != nil {
		return err
	}
	m.merge(from)
	return nil
}

func (m *combinedMetricsMerger) MergeOlder(value []byte) error {
	from := &aggregationpb.CombinedMetrics{}
	if err := from.UnmarshalVT(value); err != nil {
		return err
	}
	m.merge(from)
	return nil
}

func (m *combinedMetricsMerger) Finish(includesBase bool) ([]byte, io.Closer, error) {
	pb := m.metrics.ToProto()
	data, err := pb.MarshalVT()
	return data, nil, err
}

func (m *combinedMetricsMerger) merge(from *aggregationpb.CombinedMetrics) {
	// We merge the below fields irrespective of the services present
	// because it is possible for services to be empty if the event
	// does not fit the criteria for aggregations.
	m.metrics.EventsTotal += from.EventsTotal
	if m.metrics.YoungestEventTimestamp < from.YoungestEventTimestamp {
		m.metrics.YoungestEventTimestamp = from.YoungestEventTimestamp
	}
	// If there is overflow due to max services in either of the buckets being
	// merged then we can merge the overflow buckets without considering any
	// other scenarios.
	if len(from.OverflowServicesEstimator) > 0 {
		mergeOverflow(&m.metrics.OverflowServices, from.OverflowServices)
		mergeEstimator(
			&m.metrics.OverflowServicesEstimator,
			hllSketch(from.OverflowServicesEstimator),
		)
	}

	if len(from.ServiceMetrics) == 0 {
		return
	}
	if m.metrics.Services == nil {
		m.metrics.Services = make(map[serviceAggregationKey]serviceMetrics)
	}

	// Iterate over the services in the _from_ combined metrics and merge them
	// into the _to_ combined metrics as per the following rules:
	// 1. If the service in the _from_ bucket is also present in the _to_
	//    bucket then merge them.
	// 2. If the service in the _from_ bucket is not in the _to_ bucket:
	//    2.a. If the _to_ bucket hasn't breached the max services limit then
	//         create a new service in _to_ bucket and merge.
	//    2.b. Else, merge the _from_ bucket to the overflow service bucket
	//         of the _to_ combined metrics.
	for i := range from.ServiceMetrics {
		fromSvc := from.ServiceMetrics[i]
		serviceKeyHash := protohash.HashServiceAggregationKey(xxhash.Digest{}, fromSvc.Key)
		var sk serviceAggregationKey
		sk.FromProto(fromSvc.Key)
		toSvc, svcOverflow := getServiceMetrics(&m.metrics, sk, m.limits.MaxServices)
		if svcOverflow {
			mergeOverflow(&m.metrics.OverflowServices, fromSvc.Metrics.OverflowGroups)
			mergeToOverflowFromServiceMetrics(&m.metrics.OverflowServices, fromSvc.Metrics, serviceKeyHash)
			insertHash(&m.metrics.OverflowServicesEstimator, serviceKeyHash.Sum64())
			continue
		}
		if fromSvc.Metrics != nil {
			mergeOverflow(&toSvc.OverflowGroups, fromSvc.Metrics.OverflowGroups)
			mergeTransactionGroups(
				toSvc.TransactionGroups,
				fromSvc.Metrics.TransactionMetrics,
				constraint.New(
					len(toSvc.TransactionGroups),
					m.limits.MaxTransactionGroupsPerService,
				),
				m.constraints.totalTransactionGroups,
				serviceKeyHash,
				&toSvc.OverflowGroups.OverflowTransaction,
			)
			mergeServiceTransactionGroups(
				toSvc.ServiceTransactionGroups,
				fromSvc.Metrics.ServiceTransactionMetrics,
				constraint.New(
					len(toSvc.ServiceTransactionGroups),
					m.limits.MaxServiceTransactionGroupsPerService,
				),
				m.constraints.totalServiceTransactionGroups,
				serviceKeyHash,
				&toSvc.OverflowGroups.OverflowServiceTransaction,
			)
			mergeSpanGroups(
				toSvc.SpanGroups,
				fromSvc.Metrics.SpanMetrics,
				constraint.New(
					len(toSvc.SpanGroups),
					m.limits.MaxSpanGroupsPerService,
				),
				m.constraints.totalSpanGroups,
				serviceKeyHash,
				&toSvc.OverflowGroups.OverflowSpan,
			)
		}
		m.metrics.Services[sk] = toSvc
	}
}

// mergeTransactionGroups merges transaction aggregation groups for two combined metrics
// considering max transaction groups and max transaction groups per service limits.
func mergeTransactionGroups(
	to map[transactionAggregationKey]*aggregationpb.KeyedTransactionMetrics,
	from []*aggregationpb.KeyedTransactionMetrics,
	perSvcConstraint, globalConstraint *constraint.Constraint,
	hash xxhash.Digest,
	overflowTo *overflowTransaction,
) {
	for i := range from {
		fromTxn := from[i]
		var tk transactionAggregationKey
		tk.FromProto(fromTxn.Key)
		toTxn, ok := to[tk]
		if !ok {
			overflowed := perSvcConstraint.Maxed() || globalConstraint.Maxed()
			if overflowed {
				fromTxnKeyHash := protohash.HashTransactionAggregationKey(hash, fromTxn.Key)
				overflowTo.Merge(fromTxn.Metrics, fromTxnKeyHash.Sum64())
				continue
			}
			perSvcConstraint.Add(1)
			globalConstraint.Add(1)

			to[tk] = fromTxn.CloneVT()
			continue
		}
		mergeKeyedTransactionMetrics(toTxn, fromTxn)
	}
}

// mergeServiceTransactionGroups merges service transaction aggregation groups for two
// combined metrics considering max service transaction groups and max service
// transaction groups per service limits.
func mergeServiceTransactionGroups(
	to map[serviceTransactionAggregationKey]*aggregationpb.KeyedServiceTransactionMetrics,
	from []*aggregationpb.KeyedServiceTransactionMetrics,
	perSvcConstraint, globalConstraint *constraint.Constraint,
	hash xxhash.Digest,
	overflowTo *overflowServiceTransaction,
) {
	for i := range from {
		fromSvcTxn := from[i]
		var stk serviceTransactionAggregationKey
		stk.FromProto(fromSvcTxn.Key)
		toSvcTxn, ok := to[stk]
		if !ok {
			overflowed := perSvcConstraint.Maxed() || globalConstraint.Maxed()
			if overflowed {
				fromSvcTxnKeyHash := protohash.HashServiceTransactionAggregationKey(hash, fromSvcTxn.Key)
				overflowTo.Merge(fromSvcTxn.Metrics, fromSvcTxnKeyHash.Sum64())
				continue
			}
			perSvcConstraint.Add(1)
			globalConstraint.Add(1)

			to[stk] = fromSvcTxn.CloneVT()
			continue
		}
		mergeKeyedServiceTransactionMetrics(toSvcTxn, fromSvcTxn)
	}
}

// mergeSpanGroups merges span aggregation groups for two combined metrics considering
// max span groups and max span groups per service limits.
func mergeSpanGroups(
	to map[spanAggregationKey]*aggregationpb.KeyedSpanMetrics,
	from []*aggregationpb.KeyedSpanMetrics,
	perSvcConstraint, globalConstraint *constraint.Constraint,
	hash xxhash.Digest,
	overflowTo *overflowSpan,
) {
	for i := range from {
		fromSpan := from[i]
		var spk spanAggregationKey
		spk.FromProto(fromSpan.Key)
		toSpan, ok := to[spk]
		if !ok {
			// Protect against agents that send high cardinality span names by dropping
			// span.name if more than half of the per svc span group limit is reached.
			originalSpanName := fromSpan.Key.SpanName
			half := perSvcConstraint.Limit() / 2
			if perSvcConstraint.Value() >= half {
				spk.SpanName = ""
				fromSpan.Key.SpanName = ""
				toSpan, ok = to[spk]
			}
			if !ok {
				overflowed := perSvcConstraint.Maxed() || globalConstraint.Maxed()
				if overflowed {
					// Restore span name in case it was dropped above,
					// for cardinality estimation.
					fromSpan.Key.SpanName = originalSpanName
					fromSpanKeyHash := protohash.HashSpanAggregationKey(hash, fromSpan.Key)
					overflowTo.Merge(fromSpan.Metrics, fromSpanKeyHash.Sum64())
					continue
				}
				perSvcConstraint.Add(1)
				globalConstraint.Add(1)

				to[spk] = fromSpan.CloneVT()
				continue
			}
		}
		mergeKeyedSpanMetrics(toSpan, fromSpan)
	}
}

func mergeToOverflowFromServiceMetrics(
	to *overflow,
	from *aggregationpb.ServiceMetrics,
	hash xxhash.Digest,
) {
	if from == nil {
		return
	}
	for _, ktm := range from.TransactionMetrics {
		ktmKeyHash := protohash.HashTransactionAggregationKey(hash, ktm.Key)
		to.OverflowTransaction.Merge(ktm.Metrics, ktmKeyHash.Sum64())
	}
	for _, kstm := range from.ServiceTransactionMetrics {
		kstmKeyHash := protohash.HashServiceTransactionAggregationKey(hash, kstm.Key)
		to.OverflowServiceTransaction.Merge(kstm.Metrics, kstmKeyHash.Sum64())
	}
	for _, ksm := range from.SpanMetrics {
		ksmKeyHash := protohash.HashSpanAggregationKey(hash, ksm.Key)
		to.OverflowSpan.Merge(ksm.Metrics, ksmKeyHash.Sum64())
	}
}

func mergeOverflow(
	to *overflow,
	fromproto *aggregationpb.Overflow,
) {
	if fromproto == nil {
		return
	}
	var from overflow
	from.FromProto(fromproto)
	to.OverflowTransaction.MergeOverflow(&from.OverflowTransaction)
	to.OverflowServiceTransaction.MergeOverflow(&from.OverflowServiceTransaction)
	to.OverflowSpan.MergeOverflow(&from.OverflowSpan)
}

func mergeKeyedTransactionMetrics(
	to, from *aggregationpb.KeyedTransactionMetrics,
) {
	if from.Metrics == nil {
		return
	}
	if to.Metrics == nil {
		to.Metrics = &aggregationpb.TransactionMetrics{}
	}
	mergeTransactionMetrics(to.Metrics, from.Metrics)
}

func mergeTransactionMetrics(
	to, from *aggregationpb.TransactionMetrics,
) {
	if to.Histogram == nil && from.Histogram != nil {
		to.Histogram = &aggregationpb.HDRHistogram{}
	}
	if to.Histogram != nil && from.Histogram != nil {
		mergeHistogram(to.Histogram, from.Histogram)
	}
}

func mergeKeyedServiceTransactionMetrics(
	to, from *aggregationpb.KeyedServiceTransactionMetrics,
) {
	if from.Metrics == nil {
		return
	}
	if to.Metrics == nil {
		to.Metrics = &aggregationpb.ServiceTransactionMetrics{}
	}
	mergeServiceTransactionMetrics(to.Metrics, from.Metrics)
}

func mergeServiceTransactionMetrics(
	to, from *aggregationpb.ServiceTransactionMetrics,
) {
	if to.Histogram == nil && from.Histogram != nil {
		to.Histogram = &aggregationpb.HDRHistogram{}
	}
	if to.Histogram != nil && from.Histogram != nil {
		mergeHistogram(to.Histogram, from.Histogram)
	}
	to.FailureCount += from.FailureCount
	to.SuccessCount += from.SuccessCount
}

func mergeKeyedSpanMetrics(to, from *aggregationpb.KeyedSpanMetrics) {
	if from.Metrics == nil {
		return
	}
	if to.Metrics == nil {
		to.Metrics = &aggregationpb.SpanMetrics{}
	}
	mergeSpanMetrics(to.Metrics, from.Metrics)
}

func mergeSpanMetrics(to, from *aggregationpb.SpanMetrics) {
	to.Count += from.Count
	to.Sum += from.Sum
}

// mergeHistogram merges two proto representation of HDRHistogram. The
// merge assumes both histograms are created with same arguments and
// their representations are sorted by bucket.
func mergeHistogram(to, from *aggregationpb.HDRHistogram) {
	if len(from.Buckets) == 0 {
		return
	}

	if len(to.Buckets) == 0 {
		to.Buckets = append(to.Buckets, from.Buckets...)
		to.Counts = append(to.Counts, from.Counts...)
		return
	}

	startToIdx, found := sort.Find(len(to.Buckets), func(i int) int {
		return int(from.Buckets[0] - to.Buckets[i])
	})
	if found && len(from.Buckets) == 1 {
		// optimize for single value of `from` also found in `to`
		to.Counts[startToIdx] += from.Counts[0]
		return
	}

	// Since all values of `from` must be greater than the first value, we can
	// limit the search space in `to` to [startToIdx, len(to.Buckets))
	requiredLen := len(to.Buckets) + len(from.Buckets)
	for toIdx, fromIdx := startToIdx, 0; toIdx < len(to.Buckets) && fromIdx < len(from.Buckets); {
		v := to.Buckets[toIdx] - from.Buckets[fromIdx]
		switch {
		case v == 0:
			// For every bucket that is common, we need one less bucket in final slice
			requiredLen--
			toIdx++
			fromIdx++
		case v < 0:
			toIdx++
		case v > 0:
			fromIdx++
		}
	}

	toIdx, fromIdx := len(to.Buckets)-1, len(from.Buckets)-1
	to.Buckets = slices.Grow(to.Buckets, requiredLen-len(to.Buckets))[:requiredLen]
	to.Counts = slices.Grow(to.Counts, requiredLen-len(to.Counts))[:requiredLen]
	for idx := len(to.Buckets) - 1; idx >= 0; idx-- {
		if fromIdx < 0 {
			break
		}
		if toIdx < startToIdx {
			copy(to.Counts[startToIdx:idx+1], from.Counts[0:fromIdx+1])
			copy(to.Buckets[startToIdx:idx+1], from.Buckets[0:fromIdx+1])
			break
		}
		v := to.Buckets[toIdx] - from.Buckets[fromIdx]
		switch {
		case v == 0:
			to.Counts[idx] = to.Counts[toIdx] + from.Counts[fromIdx]
			to.Buckets[idx] = to.Buckets[toIdx]
			toIdx--
			fromIdx--
		case v > 0:
			to.Counts[idx] = to.Counts[toIdx]
			to.Buckets[idx] = to.Buckets[toIdx]
			toIdx--
		case v < 0:
			to.Counts[idx] = from.Counts[fromIdx]
			to.Buckets[idx] = from.Buckets[fromIdx]
			fromIdx--
		}
	}
}

// getServiceMetrics returns the service metric from a combined metrics based on the
// service key argument, creating one if needed. A second bool return value indicates
// if a service is returned or no service can be created due to max svcs limit breach.
func getServiceMetrics(cm *combinedMetrics, svcKey serviceAggregationKey, maxSvcs int) (serviceMetrics, bool) {
	srcSvc, ok := cm.Services[svcKey]
	if !ok {
		if len(cm.Services) < maxSvcs {
			return newServiceMetrics(), false
		}
		return serviceMetrics{}, true
	}
	return srcSvc, false
}

func newServiceMetrics() serviceMetrics {
	return serviceMetrics{
		TransactionGroups:        make(map[transactionAggregationKey]*aggregationpb.KeyedTransactionMetrics),
		ServiceTransactionGroups: make(map[serviceTransactionAggregationKey]*aggregationpb.KeyedServiceTransactionMetrics),
		SpanGroups:               make(map[spanAggregationKey]*aggregationpb.KeyedSpanMetrics),
	}
}

// constraints is a group of constraints to be observed during merge operations.
type constraints struct {
	totalTransactionGroups        *constraint.Constraint
	totalServiceTransactionGroups *constraint.Constraint
	totalSpanGroups               *constraint.Constraint
}

func newConstraints(limits Limits) constraints {
	return constraints{
		totalTransactionGroups:        constraint.New(0, limits.MaxTransactionGroups),
		totalServiceTransactionGroups: constraint.New(0, limits.MaxServiceTransactionGroups),
		totalSpanGroups:               constraint.New(0, limits.MaxSpanGroups),
	}
}
