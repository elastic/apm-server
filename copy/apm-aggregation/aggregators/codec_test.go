// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-aggregation/aggregators/internal/hdrhistogram"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestCombinedMetricsKey(t *testing.T) {
	expected := CombinedMetricsKey{
		Interval:       time.Minute,
		ProcessingTime: time.Now().Truncate(time.Minute),
		ID:             EncodeToCombinedMetricsKeyID(t, "ab01"),
	}
	data := make([]byte, CombinedMetricsKeyEncodedSize)
	assert.NoError(t, expected.MarshalBinaryToSizedBuffer(data))
	var actual CombinedMetricsKey
	assert.NoError(t, (&actual).UnmarshalBinary(data))
	assert.Empty(t, cmp.Diff(expected, actual))
}

func TestGetEncodedCombinedMetricsKeyWithoutPartitionID(t *testing.T) {
	key := CombinedMetricsKey{
		Interval:       time.Minute,
		ProcessingTime: time.Now().Truncate(time.Minute),
		ID:             EncodeToCombinedMetricsKeyID(t, "ab01"),
		PartitionID:    11,
	}
	var encoded [CombinedMetricsKeyEncodedSize]byte
	assert.NoError(t, key.MarshalBinaryToSizedBuffer(encoded[:]))

	key.PartitionID = 0
	var expected [CombinedMetricsKeyEncodedSize]byte
	assert.NoError(t, key.MarshalBinaryToSizedBuffer(expected[:]))

	assert.Equal(
		t,
		expected[:],
		GetEncodedCombinedMetricsKeyWithoutPartitionID(encoded[:]),
	)
}

func TestGlobalLabels(t *testing.T) {
	expected := globalLabels{
		Labels: map[string]*modelpb.LabelValue{
			"lb01": {
				Values: []string{"test01", "test02"},
				Global: true,
			},
		},
		NumericLabels: map[string]*modelpb.NumericLabelValue{
			"nlb01": {
				Values: []float64{0.1, 0.2},
				Global: true,
			},
		},
	}
	str, err := expected.MarshalString()
	assert.NoError(t, err)
	var actual globalLabels
	assert.NoError(t, actual.UnmarshalString(str))
	assert.Empty(t, cmp.Diff(
		expected, actual,
		cmpopts.IgnoreUnexported(
			modelpb.LabelValue{},
			modelpb.NumericLabelValue{},
		),
	))
}

func TestHistogramRepresentation(t *testing.T) {
	expected := hdrhistogram.New()
	expected.RecordDuration(time.Minute, 2)

	actual := hdrhistogram.New()
	histogramFromProto(actual, histogramToProto(expected))
	assert.Empty(t, cmp.Diff(
		expected, actual,
		cmp.Comparer(func(a, b hdrhistogram.HybridCountsRep) bool {
			return a.Equal(&b)
		}),
	))
}

func BenchmarkCombinedMetricsEncoding(b *testing.B) {
	b.ReportAllocs()
	ts := time.Now()
	cardinality := 10
	tcm := NewTestCombinedMetrics()
	sm := tcm.AddServiceMetrics(serviceAggregationKey{
		Timestamp:   ts,
		ServiceName: "bench",
	})
	for i := 0; i < cardinality; i++ {
		txnName := fmt.Sprintf("txn%d", i)
		txnType := fmt.Sprintf("typ%d", i)
		spanName := fmt.Sprintf("spn%d", i)

		sm.AddTransaction(transactionAggregationKey{
			TransactionName: txnName,
			TransactionType: txnType,
		}, WithTransactionCount(200))
		sm.AddServiceTransaction(serviceTransactionAggregationKey{
			TransactionType: txnType,
		}, WithTransactionCount(200))
		sm.AddSpan(spanAggregationKey{
			SpanName: spanName,
		})
	}
	cm := tcm.Get()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmproto := cm.ToProto()
		cmproto.ReturnToVTPool()
	}
}

func EncodeToCombinedMetricsKeyID(tb testing.TB, s string) [16]byte {
	var b [16]byte
	if len(s) > len(b) {
		tb.Fatal("invalid key length passed")
	}
	copy(b[len(b)-len(s):], s)
	return b
}
