// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"time"

	"github.com/axiomhq/hyperloglog"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/nullable"
	"github.com/elastic/apm-data/model/modelpb"
)

// Limits define the aggregation limits. Once the limits are reached
// the metrics will overflow into dedicated overflow buckets.
type Limits struct {
	// MaxServices is the limit on the total number of unique services.
	// A unique service is identified by a unique ServiceAggregationKey.
	// This limit is shared across all aggregation metrics.
	MaxServices int

	// MaxSpanGroups is the limit on total number of unique span groups
	// across all services.
	// A unique span group is identified by a unique
	// ServiceAggregationKey + SpanAggregationKey.
	MaxSpanGroups int

	// MaxSpanGroupsPerService is the limit on the total number of unique
	// span groups within a service.
	// A unique span group within a service is identified by a unique
	// SpanAggregationKey.
	MaxSpanGroupsPerService int

	// MaxTransactionGroups is the limit on total number of unique
	// transaction groups across all services.
	// A unique transaction group is identified by a unique
	// ServiceAggregationKey + TransactionAggregationKey.
	MaxTransactionGroups int

	// MaxTransactionGroupsPerService is the limit on the number of unique
	// transaction groups within a service.
	// A unique transaction group within a service is identified by a unique
	// TransactionAggregationKey.
	MaxTransactionGroupsPerService int

	// MaxServiceTransactionGroups is the limit on total number of unique
	// service transaction groups across all services.
	// A unique service transaction group is identified by a unique
	// ServiceAggregationKey + ServiceTransactionAggregationKey.
	MaxServiceTransactionGroups int

	// MaxServiceTransactionGroupsPerService is the limit on the number
	// of unique service transaction groups within a service.
	// A unique service transaction group within a service is identified
	// by a unique ServiceTransactionAggregationKey.
	MaxServiceTransactionGroupsPerService int
}

// CombinedMetricsKey models the key to store the data in LSM tree.
// Each key-value pair represents a set of unique metric for a combined metrics ID.
// The processing time used in the key should be rounded to the
// duration of aggregation since the zero time.
type CombinedMetricsKey struct {
	Interval       time.Duration
	ProcessingTime time.Time
	PartitionID    uint16
	ID             [16]byte
}

// globalLabels is an intermediate struct used to marshal/unmarshal the
// provided global labels into a comparable format. The format is used by
// pebble db to compare service aggregation keys.
type globalLabels struct {
	Labels        modelpb.Labels
	NumericLabels modelpb.NumericLabels
}

// combinedMetrics models the value to store the data in LSM tree.
// Each unique combined metrics ID stores a combined metrics per aggregation
// interval. combinedMetrics encapsulates the aggregated metrics
// as well as the overflow metrics.
type combinedMetrics struct {
	Services map[serviceAggregationKey]serviceMetrics

	// OverflowServices provides a dedicated bucket for collecting
	// aggregate metrics for all the aggregation groups for all services
	// that overflowed due to max services limit being reached.
	OverflowServices overflow

	// OverflowServicesEstimator estimates the number of unique service
	// aggregation keys that overflowed due to max services limit.
	OverflowServicesEstimator *hyperloglog.Sketch

	// EventsTotal is the total number of individual events, including
	// all overflows, that were aggregated for this combined metrics. It
	// is used for internal monitoring purposes and is approximated when
	// partitioning is enabled.
	EventsTotal float64

	// YoungestEventTimestamp is the youngest event that was aggregated
	// in the combined metrics based on the received timestamp.
	YoungestEventTimestamp uint64
}

// serviceAggregationKey models the key used to store service specific
// aggregation metrics.
type serviceAggregationKey struct {
	Timestamp           time.Time
	ServiceName         string
	ServiceEnvironment  string
	ServiceLanguageName string
	AgentName           string
	GlobalLabelsStr     string
}

// serviceMetrics models the value to store all the aggregated metrics
// for a specific service aggregation key.
type serviceMetrics struct {
	OverflowGroups           overflow
	TransactionGroups        map[transactionAggregationKey]*aggregationpb.KeyedTransactionMetrics
	ServiceTransactionGroups map[serviceTransactionAggregationKey]*aggregationpb.KeyedServiceTransactionMetrics
	SpanGroups               map[spanAggregationKey]*aggregationpb.KeyedSpanMetrics
}

func insertHash(to **hyperloglog.Sketch, hash uint64) {
	if *to == nil {
		*to = hyperloglog.New14()
	}
	(*to).InsertHash(hash)
}

func mergeEstimator(to **hyperloglog.Sketch, from *hyperloglog.Sketch) {
	if *to == nil {
		*to = hyperloglog.New14()
	}
	// Ignoring returned error here since the error is only returned if
	// the precision is set outside bounds which is not possible for our case.
	(*to).Merge(from)
}

type overflowTransaction struct {
	Metrics   *aggregationpb.TransactionMetrics
	Estimator *hyperloglog.Sketch
}

func (o *overflowTransaction) Merge(
	from *aggregationpb.TransactionMetrics,
	hash uint64,
) {
	if o.Metrics == nil {
		o.Metrics = &aggregationpb.TransactionMetrics{}
	}
	mergeTransactionMetrics(o.Metrics, from)
	insertHash(&o.Estimator, hash)
}

func (o *overflowTransaction) MergeOverflow(from *overflowTransaction) {
	if from.Estimator != nil {
		if o.Metrics == nil {
			o.Metrics = &aggregationpb.TransactionMetrics{}
		}
		mergeTransactionMetrics(o.Metrics, from.Metrics)
		mergeEstimator(&o.Estimator, from.Estimator)
	}
}

func (o *overflowTransaction) Empty() bool {
	return o.Estimator == nil
}

type overflowServiceTransaction struct {
	Metrics   *aggregationpb.ServiceTransactionMetrics
	Estimator *hyperloglog.Sketch
}

func (o *overflowServiceTransaction) Merge(
	from *aggregationpb.ServiceTransactionMetrics,
	hash uint64,
) {
	if o.Metrics == nil {
		o.Metrics = &aggregationpb.ServiceTransactionMetrics{}
	}
	mergeServiceTransactionMetrics(o.Metrics, from)
	insertHash(&o.Estimator, hash)
}

func (o *overflowServiceTransaction) MergeOverflow(from *overflowServiceTransaction) {
	if from.Estimator != nil {
		if o.Metrics == nil {
			o.Metrics = &aggregationpb.ServiceTransactionMetrics{}
		}
		mergeServiceTransactionMetrics(o.Metrics, from.Metrics)
		mergeEstimator(&o.Estimator, from.Estimator)
	}
}

func (o *overflowServiceTransaction) Empty() bool {
	return o.Estimator == nil
}

type overflowSpan struct {
	Metrics   *aggregationpb.SpanMetrics
	Estimator *hyperloglog.Sketch
}

func (o *overflowSpan) Merge(
	from *aggregationpb.SpanMetrics,
	hash uint64,
) {
	if o.Metrics == nil {
		o.Metrics = &aggregationpb.SpanMetrics{}
	}
	mergeSpanMetrics(o.Metrics, from)
	insertHash(&o.Estimator, hash)
}

func (o *overflowSpan) MergeOverflow(from *overflowSpan) {
	if from.Estimator != nil {
		if o.Metrics == nil {
			o.Metrics = &aggregationpb.SpanMetrics{}
		}
		mergeSpanMetrics(o.Metrics, from.Metrics)
		mergeEstimator(&o.Estimator, from.Estimator)
	}
}

func (o *overflowSpan) Empty() bool {
	return o.Estimator == nil
}

// overflow contains transaction and spans overflow metrics and cardinality
// estimators for the aggregation group for overflow buckets.
type overflow struct {
	OverflowTransaction        overflowTransaction
	OverflowServiceTransaction overflowServiceTransaction
	OverflowSpan               overflowSpan
}

// transactionAggregationKey models the key used to store transaction
// aggregation metrics.
type transactionAggregationKey struct {
	TraceRoot bool

	ContainerID       string
	KubernetesPodName string

	ServiceVersion  string
	ServiceNodeName string

	ServiceRuntimeName     string
	ServiceRuntimeVersion  string
	ServiceLanguageVersion string

	HostHostname   string
	HostName       string
	HostOSPlatform string

	EventOutcome string

	TransactionName   string
	TransactionType   string
	TransactionResult string

	FAASColdstart   nullable.Bool
	FAASID          string
	FAASName        string
	FAASVersion     string
	FAASTriggerType string

	CloudProvider         string
	CloudRegion           string
	CloudAvailabilityZone string
	CloudServiceName      string
	CloudAccountID        string
	CloudAccountName      string
	CloudMachineType      string
	CloudProjectID        string
	CloudProjectName      string
}

// spanAggregationKey models the key used to store span aggregation metrics.
type spanAggregationKey struct {
	SpanName string
	Outcome  string

	TargetType string
	TargetName string

	Resource string
}

// serviceTransactionAggregationKey models the key used to store
// service transaction aggregation metrics.
type serviceTransactionAggregationKey struct {
	TransactionType string
}
