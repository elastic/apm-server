// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

// TODO(lahsivjar): Add a test using reflect to validate if all
// fields are properly set.

import (
	"encoding/binary"
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/axiomhq/hyperloglog"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/hdrhistogram"
	"github.com/elastic/apm-aggregation/aggregators/nullable"
	"github.com/elastic/apm-data/model/modelpb"
)

// CombinedMetricsKeyEncodedSize gives the encoded size gives the size of
// CombinedMetricsKey in bytes. The size is used as follows:
// - 2 bytes for interval encoding
// - 8 bytes for timestamp encoding
// - 16 bytes for ID encoding
// - 2 bytes for partition ID
const CombinedMetricsKeyEncodedSize = 28

// MarshalBinaryToSizedBuffer will marshal the combined metrics key into
// its binary representation. The encoded byte slice will be used as a
// key in pebbledb. To ensure efficient sorting and time range based
// query, the first 2 bytes of the encoded slice is the aggregation
// interval, the next 8 bytes of the encoded slice is the processing time
// followed by combined metrics ID, the last 2 bytes is the partition ID.
// The binary representation ensures that all entries are ordered by the
// ID first and then ordered by the partition ID.
func (k *CombinedMetricsKey) MarshalBinaryToSizedBuffer(data []byte) error {
	ivlSeconds := uint16(k.Interval.Seconds())
	if len(data) != CombinedMetricsKeyEncodedSize {
		return errors.New("failed to marshal due to incorrect sized buffer")
	}
	var offset int

	binary.BigEndian.PutUint16(data[offset:], ivlSeconds)
	offset += 2

	binary.BigEndian.PutUint64(data[offset:], uint64(k.ProcessingTime.Unix()))
	offset += 8

	copy(data[offset:], k.ID[:])
	offset += 16

	binary.BigEndian.PutUint16(data[offset:], k.PartitionID)
	return nil
}

// UnmarshalBinary will convert the byte encoded data into CombinedMetricsKey.
func (k *CombinedMetricsKey) UnmarshalBinary(data []byte) error {
	if len(data) < 12 {
		return errors.New("invalid encoded data of insufficient length")
	}
	var offset int
	k.Interval = time.Duration(binary.BigEndian.Uint16(data[offset:2])) * time.Second
	offset += 2

	k.ProcessingTime = time.Unix(int64(binary.BigEndian.Uint64(data[offset:offset+8])), 0)
	offset += 8

	copy(k.ID[:], data[offset:offset+len(k.ID)])
	offset += len(k.ID)

	k.PartitionID = binary.BigEndian.Uint16(data[offset:])
	return nil
}

// SizeBinary returns the size of the byte array required to encode
// combined metrics key. Encoded size for CombinedMetricsKey is constant
// and alternatively the constant CombinedMetricsKeyEncodedSize can be used.
func (k *CombinedMetricsKey) SizeBinary() int {
	return CombinedMetricsKeyEncodedSize
}

// GetEncodedCombinedMetricsKeyWithoutPartitionID is a util function to
// remove partition bits from an encoded CombinedMetricsKey.
func GetEncodedCombinedMetricsKeyWithoutPartitionID(src []byte) []byte {
	var buf [CombinedMetricsKeyEncodedSize]byte
	copy(buf[:CombinedMetricsKeyEncodedSize-2], src)
	return buf[:]
}

// ToProto converts CombinedMetrics to its protobuf representation.
func (m *combinedMetrics) ToProto() *aggregationpb.CombinedMetrics {
	pb := aggregationpb.CombinedMetrics{}
	pb.ServiceMetrics = slices.Grow(pb.ServiceMetrics, len(m.Services))[:len(m.Services)]
	var i int
	for k, m := range m.Services {
		if pb.ServiceMetrics[i] == nil {
			pb.ServiceMetrics[i] = &aggregationpb.KeyedServiceMetrics{}
		}
		pb.ServiceMetrics[i].Key = k.ToProto()
		pb.ServiceMetrics[i].Metrics = m.ToProto()
		i++
	}
	if m.OverflowServicesEstimator != nil {
		pb.OverflowServices = m.OverflowServices.ToProto()
		pb.OverflowServicesEstimator = hllBytes(m.OverflowServicesEstimator)
	}
	pb.EventsTotal = m.EventsTotal
	pb.YoungestEventTimestamp = m.YoungestEventTimestamp
	return &pb
}

// ToProto converts ServiceAggregationKey to its protobuf representation.
func (k *serviceAggregationKey) ToProto() *aggregationpb.ServiceAggregationKey {
	pb := aggregationpb.ServiceAggregationKey{}
	pb.Timestamp = modelpb.FromTime(k.Timestamp)
	pb.ServiceName = k.ServiceName
	pb.ServiceEnvironment = k.ServiceEnvironment
	pb.ServiceLanguageName = k.ServiceLanguageName
	pb.AgentName = k.AgentName
	pb.GlobalLabelsStr = []byte(k.GlobalLabelsStr)
	return &pb
}

// FromProto converts protobuf representation to ServiceAggregationKey.
func (k *serviceAggregationKey) FromProto(pb *aggregationpb.ServiceAggregationKey) {
	k.Timestamp = modelpb.ToTime(pb.Timestamp)
	k.ServiceName = pb.ServiceName
	k.ServiceEnvironment = pb.ServiceEnvironment
	k.ServiceLanguageName = pb.ServiceLanguageName
	k.AgentName = pb.AgentName
	k.GlobalLabelsStr = string(pb.GlobalLabelsStr)
}

// ToProto converts ServiceMetrics to its protobuf representation.
func (m *serviceMetrics) ToProto() *aggregationpb.ServiceMetrics {
	pb := aggregationpb.ServiceMetrics{}
	pb.OverflowGroups = m.OverflowGroups.ToProto()

	pb.TransactionMetrics = slices.Grow(pb.TransactionMetrics, len(m.TransactionGroups))
	for _, m := range m.TransactionGroups {
		pb.TransactionMetrics = append(pb.TransactionMetrics, m)
	}

	pb.ServiceTransactionMetrics = slices.Grow(pb.ServiceTransactionMetrics, len(m.ServiceTransactionGroups))
	for _, m := range m.ServiceTransactionGroups {
		pb.ServiceTransactionMetrics = append(pb.ServiceTransactionMetrics, m)
	}

	pb.SpanMetrics = slices.Grow(pb.SpanMetrics, len(m.SpanGroups))
	for _, m := range m.SpanGroups {
		pb.SpanMetrics = append(pb.SpanMetrics, m)
	}

	return &pb
}

// ToProto converts TransactionAggregationKey to its protobuf representation.
func (k *transactionAggregationKey) ToProto() *aggregationpb.TransactionAggregationKey {
	pb := aggregationpb.TransactionAggregationKey{}
	pb.TraceRoot = k.TraceRoot

	pb.ContainerId = k.ContainerID
	pb.KubernetesPodName = k.KubernetesPodName

	pb.ServiceVersion = k.ServiceVersion
	pb.ServiceNodeName = k.ServiceNodeName

	pb.ServiceRuntimeName = k.ServiceRuntimeName
	pb.ServiceRuntimeVersion = k.ServiceRuntimeVersion
	pb.ServiceLanguageVersion = k.ServiceLanguageVersion

	pb.HostHostname = k.HostHostname
	pb.HostName = k.HostName
	pb.HostOsPlatform = k.HostOSPlatform

	pb.EventOutcome = k.EventOutcome

	pb.TransactionName = k.TransactionName
	pb.TransactionType = k.TransactionType
	pb.TransactionResult = k.TransactionResult

	pb.FaasColdstart = uint32(k.FAASColdstart)
	pb.FaasId = k.FAASID
	pb.FaasName = k.FAASName
	pb.FaasVersion = k.FAASVersion
	pb.FaasTriggerType = k.FAASTriggerType

	pb.CloudProvider = k.CloudProvider
	pb.CloudRegion = k.CloudRegion
	pb.CloudAvailabilityZone = k.CloudAvailabilityZone
	pb.CloudServiceName = k.CloudServiceName
	pb.CloudAccountId = k.CloudAccountID
	pb.CloudAccountName = k.CloudAccountName
	pb.CloudMachineType = k.CloudMachineType
	pb.CloudProjectId = k.CloudProjectID
	pb.CloudProjectName = k.CloudProjectName
	return &pb
}

// FromProto converts protobuf representation to TransactionAggregationKey.
func (k *transactionAggregationKey) FromProto(pb *aggregationpb.TransactionAggregationKey) {
	k.TraceRoot = pb.TraceRoot

	k.ContainerID = pb.ContainerId
	k.KubernetesPodName = pb.KubernetesPodName

	k.ServiceVersion = pb.ServiceVersion
	k.ServiceNodeName = pb.ServiceNodeName

	k.ServiceRuntimeName = pb.ServiceRuntimeName
	k.ServiceRuntimeVersion = pb.ServiceRuntimeVersion
	k.ServiceLanguageVersion = pb.ServiceLanguageVersion

	k.HostHostname = pb.HostHostname
	k.HostName = pb.HostName
	k.HostOSPlatform = pb.HostOsPlatform

	k.EventOutcome = pb.EventOutcome

	k.TransactionName = pb.TransactionName
	k.TransactionType = pb.TransactionType
	k.TransactionResult = pb.TransactionResult

	k.FAASColdstart = nullable.Bool(pb.FaasColdstart)
	k.FAASID = pb.FaasId
	k.FAASName = pb.FaasName
	k.FAASVersion = pb.FaasVersion
	k.FAASTriggerType = pb.FaasTriggerType

	k.CloudProvider = pb.CloudProvider
	k.CloudRegion = pb.CloudRegion
	k.CloudAvailabilityZone = pb.CloudAvailabilityZone
	k.CloudServiceName = pb.CloudServiceName
	k.CloudAccountID = pb.CloudAccountId
	k.CloudAccountName = pb.CloudAccountName
	k.CloudMachineType = pb.CloudMachineType
	k.CloudProjectID = pb.CloudProjectId
	k.CloudProjectName = pb.CloudProjectName
}

// ToProto converts ServiceTransactionAggregationKey to its protobuf representation.
func (k *serviceTransactionAggregationKey) ToProto() *aggregationpb.ServiceTransactionAggregationKey {
	pb := aggregationpb.ServiceTransactionAggregationKey{}
	pb.TransactionType = k.TransactionType
	return &pb
}

// FromProto converts protobuf representation to ServiceTransactionAggregationKey.
func (k *serviceTransactionAggregationKey) FromProto(pb *aggregationpb.ServiceTransactionAggregationKey) {
	k.TransactionType = pb.TransactionType
}

// ToProto converts SpanAggregationKey to its protobuf representation.
func (k *spanAggregationKey) ToProto() *aggregationpb.SpanAggregationKey {
	pb := aggregationpb.SpanAggregationKey{}
	pb.SpanName = k.SpanName
	pb.Outcome = k.Outcome

	pb.TargetType = k.TargetType
	pb.TargetName = k.TargetName

	pb.Resource = k.Resource
	return &pb
}

// FromProto converts protobuf representation to SpanAggregationKey.
func (k *spanAggregationKey) FromProto(pb *aggregationpb.SpanAggregationKey) {
	k.SpanName = pb.SpanName
	k.Outcome = pb.Outcome

	k.TargetType = pb.TargetType
	k.TargetName = pb.TargetName

	k.Resource = pb.Resource
}

// ToProto converts Overflow to its protobuf representation.
func (o *overflow) ToProto() *aggregationpb.Overflow {
	pb := aggregationpb.Overflow{}
	if !o.OverflowTransaction.Empty() {
		pb.OverflowTransactions = o.OverflowTransaction.Metrics
		pb.OverflowTransactionsEstimator = hllBytes(o.OverflowTransaction.Estimator)
	}
	if !o.OverflowServiceTransaction.Empty() {
		pb.OverflowServiceTransactions = o.OverflowServiceTransaction.Metrics
		pb.OverflowServiceTransactionsEstimator = hllBytes(o.OverflowServiceTransaction.Estimator)
	}
	if !o.OverflowSpan.Empty() {
		pb.OverflowSpans = o.OverflowSpan.Metrics
		pb.OverflowSpansEstimator = hllBytes(o.OverflowSpan.Estimator)
	}
	return &pb
}

// FromProto converts protobuf representation to Overflow.
func (o *overflow) FromProto(pb *aggregationpb.Overflow) {
	if pb.OverflowTransactions != nil {
		o.OverflowTransaction.Estimator = hllSketch(pb.OverflowTransactionsEstimator)
		o.OverflowTransaction.Metrics = pb.OverflowTransactions
		pb.OverflowTransactions = nil
	}
	if pb.OverflowServiceTransactions != nil {
		o.OverflowServiceTransaction.Estimator = hllSketch(pb.OverflowServiceTransactionsEstimator)
		o.OverflowServiceTransaction.Metrics = pb.OverflowServiceTransactions
		pb.OverflowServiceTransactions = nil
	}
	if pb.OverflowSpans != nil {
		o.OverflowSpan.Estimator = hllSketch(pb.OverflowSpansEstimator)
		o.OverflowSpan.Metrics = pb.OverflowSpans
		pb.OverflowSpans = nil
	}
}

// ToProto converts GlobalLabels to its protobuf representation.
func (gl *globalLabels) ToProto() *aggregationpb.GlobalLabels {
	pb := aggregationpb.GlobalLabels{}

	// Keys must be sorted to ensure wire formats are deterministically generated and strings are directly comparable
	// i.e. Protobuf formats are equal if and only if the structs are equal
	pb.Labels = slices.Grow(pb.Labels, len(gl.Labels))[:len(gl.Labels)]
	var i int
	for k, v := range gl.Labels {
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

	pb.NumericLabels = slices.Grow(pb.NumericLabels, len(gl.NumericLabels))[:len(gl.NumericLabels)]
	i = 0
	for k, v := range gl.NumericLabels {
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

	return &pb
}

// FromProto converts protobuf representation to globalLabels.
func (gl *globalLabels) FromProto(pb *aggregationpb.GlobalLabels) {
	gl.Labels = make(modelpb.Labels, len(pb.Labels))
	for _, l := range pb.Labels {
		gl.Labels[l.Key] = &modelpb.LabelValue{Value: l.Value, Global: true}
		gl.Labels[l.Key].Values = slices.Grow(gl.Labels[l.Key].Values, len(l.Values))[:len(l.Values)]
		copy(gl.Labels[l.Key].Values, l.Values)
	}
	gl.NumericLabels = make(modelpb.NumericLabels, len(pb.NumericLabels))
	for _, l := range pb.NumericLabels {
		gl.NumericLabels[l.Key] = &modelpb.NumericLabelValue{Value: l.Value, Global: true}
		gl.NumericLabels[l.Key].Values = slices.Grow(gl.NumericLabels[l.Key].Values, len(l.Values))[:len(l.Values)]
		copy(gl.NumericLabels[l.Key].Values, l.Values)
	}
}

// MarshalBinary marshals globalLabels to binary using protobuf.
func (gl *globalLabels) MarshalBinary() ([]byte, error) {
	if gl.Labels == nil && gl.NumericLabels == nil {
		return nil, nil
	}
	pb := gl.ToProto()
	return pb.MarshalVT()
}

// MarshalString marshals globalLabels to string from binary using protobuf.
func (gl *globalLabels) MarshalString() (string, error) {
	b, err := gl.MarshalBinary()
	return string(b), err
}

// UnmarshalBinary unmarshals binary protobuf to globalLabels.
func (gl *globalLabels) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		gl.Labels = nil
		gl.NumericLabels = nil
		return nil
	}
	pb := aggregationpb.GlobalLabels{}
	if err := pb.UnmarshalVT(data); err != nil {
		return err
	}
	gl.FromProto(&pb)
	return nil
}

// UnmarshalString unmarshals string of binary protobuf to globalLabels.
func (gl *globalLabels) UnmarshalString(data string) error {
	return gl.UnmarshalBinary([]byte(data))
}

func histogramFromProto(h *hdrhistogram.HistogramRepresentation, pb *aggregationpb.HDRHistogram) {
	if pb == nil {
		return
	}
	h.LowestTrackableValue = pb.LowestTrackableValue
	h.HighestTrackableValue = pb.HighestTrackableValue
	h.SignificantFigures = pb.SignificantFigures
	h.CountsRep.Reset()

	for i := 0; i < len(pb.Buckets); i++ {
		h.CountsRep.Add(pb.Buckets[i], pb.Counts[i])
	}
}

func histogramToProto(h *hdrhistogram.HistogramRepresentation) *aggregationpb.HDRHistogram {
	if h == nil {
		return nil
	}
	pb := aggregationpb.HDRHistogram{}
	setHistogramProto(h, &pb)
	return &pb
}

func setHistogramProto(h *hdrhistogram.HistogramRepresentation, pb *aggregationpb.HDRHistogram) {
	pb.LowestTrackableValue = h.LowestTrackableValue
	pb.HighestTrackableValue = h.HighestTrackableValue
	pb.SignificantFigures = h.SignificantFigures
	pb.Buckets = pb.Buckets[:0]
	pb.Counts = pb.Counts[:0]
	countsLen := h.CountsRep.Len()
	if countsLen > cap(pb.Buckets) {
		pb.Buckets = make([]int32, 0, countsLen)
	}
	if countsLen > cap(pb.Counts) {
		pb.Counts = make([]int64, 0, countsLen)
	}
	h.CountsRep.ForEach(func(bucket int32, count int64) {
		pb.Buckets = append(pb.Buckets, bucket)
		pb.Counts = append(pb.Counts, count)
	})
}

func hllBytes(estimator *hyperloglog.Sketch) []byte {
	if estimator == nil {
		return nil
	}
	// Ignoring error here since error will always be nil
	b, _ := estimator.MarshalBinary()
	return b
}

// hllSketchEstimate returns hllSketch(estimator).Estimate() if estimator is
// non-nil, and zero if estimator is nil.
func hllSketchEstimate(estimator []byte) uint64 {
	if sketch := hllSketch(estimator); sketch != nil {
		return sketch.Estimate()
	}
	return 0
}

func hllSketch(estimator []byte) *hyperloglog.Sketch {
	if len(estimator) == 0 {
		return nil
	}
	var sketch hyperloglog.Sketch
	// Ignoring returned error here since the error is only returned if
	// the precision is set outside bounds which is not possible for our case.
	sketch.UnmarshalBinary(estimator)
	return &sketch
}
