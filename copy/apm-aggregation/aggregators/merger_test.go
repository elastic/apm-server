// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/hdrhistogram"
)

func TestMerge(t *testing.T) {
	ts := time.Unix(0, 0).UTC()
	for _, tc := range []struct {
		name     string
		limits   Limits
		to       func() combinedMetrics
		from     func() *aggregationpb.CombinedMetrics
		expected func() combinedMetrics
	}{
		{
			name: "no_overflow_with_count_values",
			limits: Limits{
				MaxSpanGroups:                         2,
				MaxSpanGroupsPerService:               2,
				MaxTransactionGroups:                  2,
				MaxTransactionGroupsPerService:        2,
				MaxServiceTransactionGroups:           2,
				MaxServiceTransactionGroupsPerService: 2,
				MaxServices:                           2,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(5)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(4)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(2)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(2)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(2)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
		},
		{
			name: "no_overflow_with_histograms_in_to",
			limits: Limits{
				MaxSpanGroups:                         2,
				MaxSpanGroupsPerService:               2,
				MaxTransactionGroups:                  2,
				MaxTransactionGroupsPerService:        2,
				MaxServiceTransactionGroups:           2,
				MaxServiceTransactionGroupsPerService: 2,
				MaxServices:                           2,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1000)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(500)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(500)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(500)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(4)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(2)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(2)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(2)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1004)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(502)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(502)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(502)).
					Get()
			},
		},
		{
			name: "no_overflow_with_histogram_in_from",
			limits: Limits{
				MaxSpanGroups:                         2,
				MaxSpanGroupsPerService:               2,
				MaxTransactionGroups:                  2,
				MaxTransactionGroupsPerService:        2,
				MaxServiceTransactionGroups:           2,
				MaxServiceTransactionGroupsPerService: 2,
				MaxServices:                           2,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(4)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(2)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(2)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(2)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1000)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(500)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(500)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(500)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1004)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(502)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(502)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(502)).
					Get()
			},
		},
		{
			name: "no_overflow_with_histogram_in_both",
			limits: Limits{
				MaxSpanGroups:                         2,
				MaxSpanGroupsPerService:               2,
				MaxTransactionGroups:                  2,
				MaxTransactionGroupsPerService:        2,
				MaxServiceTransactionGroups:           2,
				MaxServiceTransactionGroupsPerService: 2,
				MaxServices:                           2,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1400)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(700)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(700)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(700)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1000)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(500)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(500)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(500)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(2400)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(1200)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(1200)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(1200)).
					Get()
			},
		},
		{
			name: "per_svc_overflow_due_to_merge",
			limits: Limits{
				MaxSpanGroups:                         100,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  100,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           100,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(24)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					// no merge as span, transaction, and service transaction will overflow
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					// all span, transaction, and service transaction from _from_ will overflow
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					Get()
			},
		},
		{
			name: "global_overflow_due_to_merge",
			limits: Limits{
				MaxSpanGroups:                         1,
				MaxSpanGroupsPerService:               100,
				MaxTransactionGroups:                  1,
				MaxTransactionGroupsPerService:        100,
				MaxServiceTransactionGroups:           1,
				MaxServiceTransactionGroupsPerService: 100,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(24)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					// no merge as span, transaction, and service transaction will overflow
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					// all span, transaction, and service transaction from _from_ will overflow
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					Get()
			},
		},
		{
			name: "to_overflowed_before_merge",
			limits: Limits{
				MaxSpanGroups:                         1,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  1,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           1,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(34)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(10)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(10)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(10)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(44)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(15)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(15),
					).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(15),
					).
					Get()
			},
		},
		{
			name: "from_overflowed_before_merge",
			limits: Limits{
				MaxSpanGroups:                         1,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  1,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           1,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(26)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span3"}, WithSpanCount(8)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type3"},
						WithTransactionCount(8)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn3", TransactionType: "type3"},
						WithTransactionCount(8)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(40)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span3"}, WithSpanCount(8)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type3"},
						WithTransactionCount(8)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn3", TransactionType: "type3"},
						WithTransactionCount(8)).
					Get()
			},
		},
		{
			name: "svc_overflow",
			limits: Limits{
				MaxSpanGroups:                         1,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  1,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           1,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc2"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(5)).
					GetProto()
			},
			expected: func() combinedMetrics {
				tcm := NewTestCombinedMetrics(WithEventsTotal(24))
				tcm.
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(7))
				// svc2 overflows
				tcm.
					AddServiceMetricsOverflow(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc2"}).
					AddTransactionOverflow(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(5)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(5)).
					AddSpanOverflow(
						spanAggregationKey{SpanName: "span1"}, WithSpanCount(5))
				return tcm.Get()
			},
		},
		{
			name: "svc_overflow_only",
			limits: Limits{
				MaxSpanGroups:                         1,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  1,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           1,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(111)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(222)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc2"}).
					GetProto()
			},
			expected: func() combinedMetrics {
				tcm := NewTestCombinedMetrics(WithEventsTotal(333))
				tcm.
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					tcm.
					AddServiceMetricsOverflow(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc2"})
				return tcm.Get()
			},
		},
		{
			name: "per_svc_overflow_known_svc",
			limits: Limits{
				MaxSpanGroups:                         100,
				MaxSpanGroupsPerService:               1,
				MaxTransactionGroups:                  100,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           100,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(14)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(10)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(24)).
					AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddSpan(spanAggregationKey{SpanName: "span1"}, WithSpanCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					AddTransaction(
						transactionAggregationKey{TransactionName: "txn1", TransactionType: "type1"},
						WithTransactionCount(7)).
					AddSpanOverflow(spanAggregationKey{SpanName: "span2"}, WithSpanCount(5)).
					AddServiceTransactionOverflow(
						serviceTransactionAggregationKey{TransactionType: "type2"},
						WithTransactionCount(5)).
					AddTransactionOverflow(
						transactionAggregationKey{TransactionName: "txn2", TransactionType: "type2"},
						WithTransactionCount(5)).
					Get()
			},
		},
		{
			name: "merge_with_empty_combined_metrics",
			limits: Limits{
				MaxSpanGroups:                         100,
				MaxSpanGroupsPerService:               10,
				MaxTransactionGroups:                  100,
				MaxTransactionGroupsPerService:        1,
				MaxServiceTransactionGroups:           100,
				MaxServiceTransactionGroupsPerService: 1,
				MaxServices:                           1,
			},
			to: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(7)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddTransaction(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
			from: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(1)).GetProto()
			},
			expected: func() combinedMetrics {
				return NewTestCombinedMetrics(WithEventsTotal(8)).
					AddServiceMetrics(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
					AddTransaction(
						transactionAggregationKey{
							TransactionName: "txn1",
							TransactionType: "type1",
						}, WithTransactionCount(7)).
					AddServiceTransaction(
						serviceTransactionAggregationKey{TransactionType: "type1"},
						WithTransactionCount(7)).
					Get()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Since we start with an existing metrics in combinedMetricsMerger,
			// we'll have to make sure constraints struct is containing the correct counts.
			metrics := tc.to()
			constraints := newConstraints(tc.limits)
			for _, svc := range metrics.Services {
				constraints.totalTransactionGroups.Add(len(svc.TransactionGroups))
				constraints.totalServiceTransactionGroups.Add(len(svc.ServiceTransactionGroups))
				constraints.totalSpanGroups.Add(len(svc.SpanGroups))
			}
			cmm := combinedMetricsMerger{
				limits:      tc.limits,
				constraints: constraints,
				metrics:     metrics,
			}
			cmm.merge(tc.from())
			assert.Empty(t, cmp.Diff(
				tc.expected(), cmm.metrics,
				protocmp.Transform(),
				cmp.Exporter(func(reflect.Type) bool { return true }),
			))
		})
	}
}

func TestCardinalityEstimationOnSubKeyCollision(t *testing.T) {
	limits := Limits{
		MaxSpanGroups:                         100,
		MaxSpanGroupsPerService:               100,
		MaxTransactionGroups:                  100,
		MaxTransactionGroupsPerService:        100,
		MaxServiceTransactionGroups:           100,
		MaxServiceTransactionGroupsPerService: 100,
		MaxServices:                           1,
	}
	ts := time.Time{}
	to := NewTestCombinedMetrics(WithEventsTotal(0)).
		AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"}).
		Get()
	from1 := NewTestCombinedMetrics(WithEventsTotal(10)).
		AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc2"}).
		AddSpan(spanAggregationKey{}, WithSpanCount(5)).
		AddTransaction(transactionAggregationKey{
			TransactionName: "txn1",
			TransactionType: "type1",
		}, WithTransactionCount(5)).
		AddServiceTransaction(serviceTransactionAggregationKey{
			TransactionType: "type1",
		}, WithTransactionCount(5)).
		GetProto()
	from2 := NewTestCombinedMetrics(WithEventsTotal(10)).
		AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc3"}).
		AddSpan(spanAggregationKey{}, WithSpanCount(5)).
		AddTransaction(transactionAggregationKey{
			TransactionName: "txn1",
			TransactionType: "type1",
		}, WithTransactionCount(5)).
		AddServiceTransaction(serviceTransactionAggregationKey{
			TransactionType: "type1",
		}, WithTransactionCount(5)).
		GetProto()
	cmm := combinedMetricsMerger{
		limits:  limits,
		metrics: to,
	}
	cmm.merge(from1)
	cmm.merge(from2)
	assert.Equal(t, uint64(2), cmm.metrics.OverflowServices.OverflowTransaction.Estimator.Estimate())
	assert.Equal(t, uint64(2), cmm.metrics.OverflowServices.OverflowServiceTransaction.Estimator.Estimate())
	assert.Equal(t, uint64(2), cmm.metrics.OverflowServices.OverflowSpan.Estimator.Estimate())
}

func TestMergeHistogramEquiv(t *testing.T) {
	for _, tc := range []struct {
		name       string
		recordFunc func(h1, h2 *hdrhistogram.HistogramRepresentation)
	}{
		{
			name: "zero_values",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				h1.RecordValues(0, 0)
				h2.RecordValues(0, 0)
			},
		},
		{
			name: "random_only_to",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				for i := 0; i < 1_000_000; i++ {
					v := rand.Int63n(3_600_000_000)
					c := rand.Int63n(1_000)
					h1.RecordValues(v, c)
				}
			},
		},
		{
			name: "random_only_from",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				for i := 0; i < 1_000_000; i++ {
					v := rand.Int63n(3_600_000_000)
					c := rand.Int63n(1_000)
					h2.RecordValues(v, c)
				}
			},
		},
		{
			name: "random_both",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				for i := 0; i < 1_000_000; i++ {
					v1, v2 := rand.Int63n(3_600_000_000), rand.Int63n(3_600_000_000)
					c1, c2 := rand.Int63n(1_000), rand.Int63n(1_000)
					h1.RecordValues(v1, c1)
					h2.RecordValues(v2, c2)
				}
			},
		},
		// There may be special fast paths for single value from,
		// and since we get them quite often,
		// we have the following test cases for it.
		{
			name: "random_to_single_value_from_hit",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				var v, c int64
				for i := 0; i < 1_000_000; i++ {
					v = rand.Int63n(3_600_000_000)
					c = rand.Int63n(1_000)
					h1.RecordValues(v, c)
				}
				c = rand.Int63n(1_000)
				h2.RecordValues(v, c)
			},
		},
		{
			name: "random_to_single_value_from_miss",
			recordFunc: func(h1, h2 *hdrhistogram.HistogramRepresentation) {
				for i := 0; i < 1_000_000; i++ {
					v := rand.Int63n(3_600_000_000)
					c := rand.Int63n(1_000)
					h1.RecordValues(v, c)

				}
				v := rand.Int63n(3_600_000_000)
				c := rand.Int63n(1_000)
				h2.RecordValues(v, c)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Test assumes histogram representation Merge is correct
			hist1, hist2 := hdrhistogram.New(), hdrhistogram.New()

			tc.recordFunc(hist1, hist2)
			histproto1, histproto2 := histogramToProto(hist1), histogramToProto(hist2)
			hist1.Merge(hist2)
			mergeHistogram(histproto1, histproto2)
			histActual := hdrhistogram.New()
			histogramFromProto(histActual, histproto1)

			assert.Empty(t, cmp.Diff(
				hist1,
				histActual,
				cmp.AllowUnexported(hdrhistogram.HistogramRepresentation{}),
				cmp.AllowUnexported(hdrhistogram.HybridCountsRep{}),
			))
		})
	}
}

func TestMergeHistogram(t *testing.T) {
	for _, tc := range []struct {
		name     string
		to       *aggregationpb.HDRHistogram
		from     *aggregationpb.HDRHistogram
		expected *aggregationpb.HDRHistogram
	}{
		{
			name: "non_single_value",
			to: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 3, 5, 7, 9},
				Counts:  []int64{1, 3, 5, 7, 9},
			},
			from: &aggregationpb.HDRHistogram{
				Buckets: []int32{2, 4, 5, 8},
				Counts:  []int64{2, 4, 5, 8},
			},
			expected: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 2, 3, 4, 5, 7, 8, 9},
				Counts:  []int64{1, 2, 3, 4, 10, 7, 8, 9},
			},
		},
		{
			name: "empty_to",
			to: &aggregationpb.HDRHistogram{
				Buckets: []int32{},
				Counts:  []int64{},
			},
			from: &aggregationpb.HDRHistogram{
				Buckets: []int32{1},
				Counts:  []int64{1},
			},
			expected: &aggregationpb.HDRHistogram{
				Buckets: []int32{1},
				Counts:  []int64{1},
			},
		},
		{
			name: "empty_from",
			to: &aggregationpb.HDRHistogram{
				Buckets: []int32{1},
				Counts:  []int64{1},
			},
			from: &aggregationpb.HDRHistogram{
				Buckets: []int32{},
				Counts:  []int64{},
			},
			expected: &aggregationpb.HDRHistogram{
				Buckets: []int32{1},
				Counts:  []int64{1},
			},
		},
		{
			name: "single_value_from_hit",
			to: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 2, 3},
				Counts:  []int64{1, 2, 3},
			},
			from: &aggregationpb.HDRHistogram{
				Buckets: []int32{3},
				Counts:  []int64{4},
			},
			expected: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 2, 3},
				Counts:  []int64{1, 2, 7},
			},
		},
		{
			name: "single_value_from_miss",
			to: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 2, 4},
				Counts:  []int64{1, 2, 4},
			},
			from: &aggregationpb.HDRHistogram{
				Buckets: []int32{3},
				Counts:  []int64{3},
			},
			expected: &aggregationpb.HDRHistogram{
				Buckets: []int32{1, 2, 3, 4},
				Counts:  []int64{1, 2, 3, 4},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mergeHistogram(tc.to, tc.from)

			assert.Empty(t, cmp.Diff(
				tc.expected,
				tc.to,
				cmpopts.IgnoreUnexported(aggregationpb.HDRHistogram{}),
			))
		})
	}
}
