// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregators

import (
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators/internal/hdrhistogram"
	"github.com/elastic/apm-aggregation/aggregators/nullable"
	"github.com/elastic/apm-data/model/modelpb"
)

func TestEventToCombinedMetrics(t *testing.T) {
	ts := time.Now().UTC()
	receivedTS := ts.Add(time.Second)
	baseEvent := &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(ts),
		ParentId:  "nonroot",
		Service:   &modelpb.Service{Name: "test"},
		Event: &modelpb.Event{
			Duration: uint64(time.Second),
			Outcome:  "success",
			Received: modelpb.FromTime(receivedTS),
		},
	}
	for _, tc := range []struct {
		name       string
		input      func() []*modelpb.APMEvent
		partitions uint16
		expected   func() []*aggregationpb.CombinedMetrics
	}{
		{
			name: "nil-input",
			input: func() []*modelpb.APMEvent {
				return nil
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return nil
			},
		},
		{
			name: "with-zero-rep-count-txn",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Transaction = &modelpb.Transaction{
					Name:                "testtxn",
					Type:                "testtyp",
					RepresentativeCount: 0,
				}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						GetProto(),
				}
			},
		},
		{
			name: "with-good-txn",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Transaction = &modelpb.Transaction{
					Name:                "testtxn",
					Type:                "testtyp",
					RepresentativeCount: 1,
				}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						AddTransaction(transactionAggregationKey{
							TransactionName: "testtxn",
							TransactionType: "testtyp",
							EventOutcome:    "success",
						}).
						AddServiceTransaction(serviceTransactionAggregationKey{
							TransactionType: "testtyp",
						}).GetProto(),
				}
			},
		},
		{
			name: "with-zero-rep-count-span",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Span = &modelpb.Span{
					Name:                "testspan",
					Type:                "testtyp",
					RepresentativeCount: 0,
				}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						GetProto(),
				}
			},
		},
		{
			name: "with-no-exit-span",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Span = &modelpb.Span{
					Name:                "testspan",
					Type:                "testtyp",
					RepresentativeCount: 1,
				}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						GetProto(),
				}
			},
		},
		{
			name: "with-good-span-svc-target",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Span = &modelpb.Span{
					Name:                "testspan",
					Type:                "testtyp",
					RepresentativeCount: 1,
				}
				event.Service.Target = &modelpb.ServiceTarget{
					Name: "psql",
					Type: "db",
				}
				// Current test structs are hardcoded to use 1ns for spans
				event.Event.Duration = uint64(time.Nanosecond)
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						AddSpan(spanAggregationKey{
							SpanName:   "testspan",
							TargetName: "psql",
							TargetType: "db",
							Outcome:    "success",
						}).GetProto(),
				}
			},
		},
		{
			name: "with-good-span-dest-svc",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Span = &modelpb.Span{
					Name:                "testspan",
					Type:                "testtyp",
					RepresentativeCount: 1,
					DestinationService: &modelpb.DestinationService{
						Resource: "db",
					},
				}
				// Current test structs are hardcoded to use 1ns for spans
				event.Event.Duration = uint64(time.Nanosecond)
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						AddSpan(spanAggregationKey{
							SpanName: "testspan",
							Resource: "db",
							Outcome:  "success",
						}).GetProto(),
				}
			},
		},
		{
			name: "with-metricset",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Metricset = &modelpb.Metricset{
					Name:     "testmetricset",
					Interval: "1m",
				}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						GetProto(),
				}
			},
		},
		{
			name: "with-log",
			input: func() []*modelpb.APMEvent {
				event := baseEvent.CloneVT()
				event.Log = &modelpb.Log{}
				return []*modelpb.APMEvent{event}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						GetProto(),
				}
			},
		},
		{
			name: "with-success-txn-followed-by-unknown-txn",
			input: func() []*modelpb.APMEvent {
				success := baseEvent.CloneVT()
				success.Transaction = &modelpb.Transaction{
					Name:                "testtxn1",
					Type:                "testtyp1",
					RepresentativeCount: 1,
				}
				unknown := baseEvent.CloneVT()
				unknown.Event.Outcome = "unknown"
				unknown.Transaction = &modelpb.Transaction{
					Name:                "testtxn2",
					Type:                "testtyp2",
					RepresentativeCount: 1,
				}
				return []*modelpb.APMEvent{success, unknown}
			},
			partitions: 1,
			expected: func() []*aggregationpb.CombinedMetrics {
				return []*aggregationpb.CombinedMetrics{
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						AddTransaction(transactionAggregationKey{
							TransactionName: "testtxn1",
							TransactionType: "testtyp1",
							EventOutcome:    "success",
						}).
						AddServiceTransaction(serviceTransactionAggregationKey{
							TransactionType: "testtyp1",
						}).GetProto(),
					NewTestCombinedMetrics(
						WithEventsTotal(1),
						WithYoungestEventTimestamp(receivedTS)).
						AddServiceMetrics(serviceAggregationKey{
							Timestamp:   ts.Truncate(time.Minute),
							ServiceName: "test"}).
						AddTransaction(transactionAggregationKey{
							TransactionName: "testtxn2",
							TransactionType: "testtyp2",
							EventOutcome:    "unknown",
						}).
						AddServiceTransaction(serviceTransactionAggregationKey{
							TransactionType: "testtyp2",
						}, WithEventOutcome("unknown")).GetProto(),
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmk := CombinedMetricsKey{
				Interval:       time.Minute,
				ProcessingTime: time.Now().Truncate(time.Minute),
				ID:             EncodeToCombinedMetricsKeyID(t, "ab01"),
			}
			var actual []*aggregationpb.CombinedMetrics
			collector := func(
				_ CombinedMetricsKey,
				m *aggregationpb.CombinedMetrics,
			) error {
				actual = append(actual, m.CloneVT())
				return nil
			}
			for _, e := range tc.input() {
				err := eventToCombinedMetrics(e, cmk, tc.partitions, collector)
				require.NoError(t, err)
			}
			assert.Empty(t, cmp.Diff(
				tc.expected(), actual,
				cmp.Comparer(func(a, b hdrhistogram.HybridCountsRep) bool {
					return a.Equal(&b)
				}),
				protocmp.Transform(),
				protocmp.IgnoreEmptyMessages(),
			))
		})
	}
}

func TestCombinedMetricsToBatch(t *testing.T) {
	ts := time.Now()
	youngestEventTS := ts.Add(-time.Second)
	aggIvl := time.Minute
	processingTime := ts.Truncate(aggIvl)
	svcName := "test"
	coldstart := nullable.True
	var (
		svc            = serviceAggregationKey{Timestamp: ts, ServiceName: svcName}
		faas           = &modelpb.Faas{Id: "f1", ColdStart: coldstart.ToBoolPtr(), Version: "v2", TriggerType: "http"}
		span           = spanAggregationKey{SpanName: "spn", Resource: "postgresql"}
		overflowSpan   = spanAggregationKey{TargetName: "_other"}
		spanCount      = 1
		svcTxn         = serviceTransactionAggregationKey{TransactionType: "typ"}
		overflowSvcTxn = serviceTransactionAggregationKey{TransactionType: "_other"}
		txn            = transactionAggregationKey{TransactionName: "txn", TransactionType: "typ"}
		txnFaas        = transactionAggregationKey{TransactionName: "txn", TransactionType: "typ",
			FAASID: faas.Id, FAASColdstart: coldstart, FAASVersion: faas.Version, FAASTriggerType: faas.TriggerType}
		overflowTxn = transactionAggregationKey{TransactionName: "_other"}
		txnCount    = 100
	)
	for _, tc := range []struct {
		name                string
		aggregationInterval time.Duration
		combinedMetrics     func() *aggregationpb.CombinedMetrics
		expectedEvents      modelpb.Batch
	}{
		{
			name:                "no_overflow_without_faas",
			aggregationInterval: aggIvl,
			combinedMetrics: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithYoungestEventTimestamp(youngestEventTS)).
					AddServiceMetrics(svc).
					AddSpan(span, WithSpanCount(spanCount)).
					AddTransaction(txn, WithTransactionCount(txnCount)).
					AddServiceTransaction(svcTxn, WithTransactionCount(txnCount)).
					GetProto()
			},
			expectedEvents: []*modelpb.APMEvent{
				createTestTransactionMetric(ts, aggIvl, svcName, txn, nil, txnCount, 0),
				createTestServiceTransactionMetric(ts, aggIvl, svcName, svcTxn, txnCount, 0),
				createTestSpanMetric(ts, aggIvl, svcName, span, spanCount, 0),
				createTestServiceSummaryMetric(ts, aggIvl, svcName, 0),
			},
		},
		{
			name:                "no_overflow",
			aggregationInterval: aggIvl,
			combinedMetrics: func() *aggregationpb.CombinedMetrics {
				return NewTestCombinedMetrics(WithYoungestEventTimestamp(youngestEventTS)).
					AddServiceMetrics(svc).
					AddSpan(span, WithSpanCount(spanCount)).
					AddTransaction(txnFaas, WithTransactionCount(txnCount)).
					AddServiceTransaction(svcTxn, WithTransactionCount(txnCount)).
					GetProto()
			},
			expectedEvents: []*modelpb.APMEvent{
				createTestTransactionMetric(ts, aggIvl, svcName, txn, faas, txnCount, 0),
				createTestServiceTransactionMetric(ts, aggIvl, svcName, svcTxn, txnCount, 0),
				createTestSpanMetric(ts, aggIvl, svcName, span, spanCount, 0),
				createTestServiceSummaryMetric(ts, aggIvl, svcName, 0),
			},
		},
		{
			name:                "overflow",
			aggregationInterval: aggIvl,
			combinedMetrics: func() *aggregationpb.CombinedMetrics {
				tcm := NewTestCombinedMetrics(WithYoungestEventTimestamp(youngestEventTS))
				tcm.
					AddServiceMetrics(svc).
					AddSpan(span, WithSpanCount(spanCount)).
					AddTransaction(txnFaas, WithTransactionCount(txnCount)).
					AddServiceTransaction(svcTxn, WithTransactionCount(txnCount)).
					AddTransactionOverflow(txn, WithTransactionCount(txnCount)).
					AddServiceTransactionOverflow(svcTxn, WithTransactionCount(txnCount)).
					AddSpanOverflow(span, WithSpanCount(spanCount))
				// Add global service overflow
				tcm.
					AddServiceMetricsOverflow(
						serviceAggregationKey{Timestamp: ts, ServiceName: "svc_overflow"})
				return tcm.GetProto()
			},
			expectedEvents: []*modelpb.APMEvent{
				createTestTransactionMetric(ts, aggIvl, svcName, txnFaas, faas, txnCount, 0),
				createTestServiceTransactionMetric(ts, aggIvl, svcName, svcTxn, txnCount, 0),
				createTestSpanMetric(ts, aggIvl, svcName, span, spanCount, 0),
				createTestServiceSummaryMetric(ts, aggIvl, svcName, 0),
				// Events due to overflow
				createTestTransactionMetric(processingTime, aggIvl, svcName, overflowTxn, nil, txnCount, 1),
				createTestServiceTransactionMetric(processingTime, aggIvl, svcName, overflowSvcTxn, txnCount, 1),
				createTestSpanMetric(processingTime, aggIvl, svcName, overflowSpan, spanCount, 1),
				createTestServiceSummaryMetric(processingTime, aggIvl, "_other", 1),
			},
		},
		{
			name:                "service_overflow",
			aggregationInterval: aggIvl,
			combinedMetrics: func() *aggregationpb.CombinedMetrics {
				tcm := NewTestCombinedMetrics(WithYoungestEventTimestamp(youngestEventTS))
				tcm.AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "svc1"})
				tcm.AddServiceMetricsOverflow(serviceAggregationKey{
					Timestamp: ts, ServiceName: "svc1",
					GlobalLabelsStr: getTestGlobalLabelsStr(t, "1"),
				})
				tcm.AddServiceMetricsOverflow(serviceAggregationKey{
					Timestamp: ts, ServiceName: "svc2",
					GlobalLabelsStr: getTestGlobalLabelsStr(t, "2"),
				})
				return tcm.GetProto()
			},
			expectedEvents: []*modelpb.APMEvent{
				createTestServiceSummaryMetric(ts, aggIvl, "svc1", 0),
				createTestServiceSummaryMetric(processingTime, aggIvl, "_other", 2),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := CombinedMetricsToBatch(
				tc.combinedMetrics(),
				processingTime,
				tc.aggregationInterval,
			)
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(
				tc.expectedEvents, *b,
				cmpopts.IgnoreTypes(netip.Addr{}),
				cmpopts.SortSlices(func(e1, e2 *modelpb.APMEvent) bool {
					m1Name := e1.GetMetricset().GetName()
					m2Name := e2.GetMetricset().GetName()
					if m1Name != m2Name {
						return m1Name < m2Name
					}

					a1Name := e1.GetAgent().GetName()
					a2Name := e2.GetAgent().GetName()
					if a1Name != a2Name {
						return a1Name < a2Name
					}

					return e1.GetService().GetEnvironment() < e2.GetService().GetEnvironment()
				}),
				protocmp.Transform(),
				protocmp.FilterField(
					&modelpb.Event{},
					"received",
					cmp.Comparer(func(a, b uint64) bool {
						if a > b {
							a, b = b, a
						}
						// The recevied timestamp is set as time.Now in both actual and
						// expected events. We assert that both these values are within
						// a threshold.
						return b-a < uint64(10*time.Second)
					}),
				),
			))
		})
	}
}

func BenchmarkCombinedMetricsToBatch(b *testing.B) {
	ai := time.Hour
	ts := time.Now()
	pt := ts.Truncate(ai)
	cardinality := 10
	tcm := NewTestCombinedMetrics().
		AddServiceMetrics(serviceAggregationKey{Timestamp: ts, ServiceName: "bench"})
	for i := 0; i < cardinality; i++ {
		txnName := fmt.Sprintf("txn%d", i)
		txnType := fmt.Sprintf("typ%d", i)
		spanName := fmt.Sprintf("spn%d", i)
		tcm.
			AddTransaction(transactionAggregationKey{
				TransactionName: txnName,
				TransactionType: txnType,
			}, WithTransactionCount(200)).
			AddServiceTransaction(serviceTransactionAggregationKey{
				TransactionType: txnType,
			}, WithTransactionCount(200)).
			AddSpan(spanAggregationKey{
				SpanName: spanName,
			})
	}
	cm := tcm.GetProto()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch, err := CombinedMetricsToBatch(cm, pt, ai)
		if err != nil {
			b.Fatal(err)
		}
		for _, e := range *batch {
			e.ReturnToVTPool()
		}
	}
}

func BenchmarkEventToCombinedMetrics(b *testing.B) {
	event := &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(time.Now()),
		ParentId:  "nonroot",
		Service: &modelpb.Service{
			Name: "test",
		},
		Event: &modelpb.Event{
			Duration: uint64(time.Second),
			Outcome:  "success",
		},
		Transaction: &modelpb.Transaction{
			RepresentativeCount: 1,
			Name:                "testtxn",
			Type:                "testtyp",
		},
	}
	cmk := CombinedMetricsKey{
		Interval:       time.Minute,
		ProcessingTime: time.Now().Truncate(time.Minute),
		ID:             EncodeToCombinedMetricsKeyID(b, "ab01"),
	}
	noop := func(_ CombinedMetricsKey, _ *aggregationpb.CombinedMetrics) error {
		return nil
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := eventToCombinedMetrics(event, cmk, 1 /*partitions*/, noop)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func createTestServiceSummaryMetric(
	ts time.Time,
	ivl time.Duration,
	svcName string,
	overflowCount int,
) *modelpb.APMEvent {
	var metricsetSamples []*modelpb.MetricsetSample
	if overflowCount > 0 {
		metricsetSamples = []*modelpb.MetricsetSample{
			{
				Name:  "service_summary.aggregation.overflow_count",
				Value: float64(overflowCount),
			},
		}
	}
	return &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(ts),
		Event:     &modelpb.Event{Received: modelpb.FromTime(time.Now())},
		Metricset: &modelpb.Metricset{
			Name:     "service_summary",
			Samples:  metricsetSamples,
			Interval: formatDuration(ivl),
		},
		Service: &modelpb.Service{Name: svcName},
	}
}

func createTestTransactionMetric(
	ts time.Time,
	ivl time.Duration,
	svcName string,
	txn transactionAggregationKey,
	faas *modelpb.Faas,
	count, overflowCount int,
) *modelpb.APMEvent {
	histRep := hdrhistogram.New()
	histRep.RecordDuration(time.Second, float64(count))
	total, counts, values := histRep.Buckets()
	var eventSuccessSummary modelpb.SummaryMetric
	switch txn.EventOutcome {
	case "success":
		eventSuccessSummary.Count = total
		eventSuccessSummary.Sum = float64(total)
	case "failure":
		eventSuccessSummary.Count = total
	case "unknown":
		// Keep both Count and Sum as 0.
	}
	transactionDurationSummary := &modelpb.SummaryMetric{
		Count: total,
		// only 1 expected element
		Sum: values[0] * float64(counts[0]),
	}
	var metricsetSamples []*modelpb.MetricsetSample
	if overflowCount > 0 {
		metricsetSamples = []*modelpb.MetricsetSample{
			{
				Name:  "transaction.aggregation.overflow_count",
				Value: float64(overflowCount),
			},
		}
	}
	return &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(ts),
		Event: &modelpb.Event{
			SuccessCount: &eventSuccessSummary,
			Received:     modelpb.FromTime(time.Now()),
		},
		Metricset: &modelpb.Metricset{
			Name:     "transaction",
			Interval: formatDuration(ivl),
			Samples:  metricsetSamples,
			DocCount: total,
		},
		Service: &modelpb.Service{Name: svcName},
		Transaction: &modelpb.Transaction{
			Name: txn.TransactionName,
			Type: txn.TransactionType,
			DurationHistogram: &modelpb.Histogram{
				Counts: counts,
				Values: values,
			},
			DurationSummary: transactionDurationSummary,
		},
		Faas: faas,
	}
}

func createTestServiceTransactionMetric(
	ts time.Time,
	ivl time.Duration,
	svcName string,
	svcTxn serviceTransactionAggregationKey,
	count, overflowCount int,
) *modelpb.APMEvent {
	histRep := hdrhistogram.New()
	histRep.RecordDuration(time.Second, float64(count))
	total, counts, values := histRep.Buckets()
	transactionDurationSummary := &modelpb.SummaryMetric{
		Count: total,
		// only 1 expected element
		Sum: values[0] * float64(counts[0]),
	}
	var metricsetSamples []*modelpb.MetricsetSample
	if overflowCount > 0 {
		metricsetSamples = []*modelpb.MetricsetSample{
			{
				Name:  "service_transaction.aggregation.overflow_count",
				Value: float64(overflowCount),
			},
		}
	}
	return &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(ts),
		Metricset: &modelpb.Metricset{
			Name:     "service_transaction",
			Interval: formatDuration(ivl),
			Samples:  metricsetSamples,
			DocCount: total,
		},
		Service: &modelpb.Service{Name: svcName},
		Transaction: &modelpb.Transaction{
			Type: svcTxn.TransactionType,
			DurationHistogram: &modelpb.Histogram{
				Counts: counts,
				Values: values,
			},
			DurationSummary: transactionDurationSummary,
		},
		Event: &modelpb.Event{
			Received: modelpb.FromTime(time.Now()),
			SuccessCount: &modelpb.SummaryMetric{
				// test code generates all success events
				Count: uint64(count),
				Sum:   float64(count),
			},
		},
	}
}

func createTestSpanMetric(
	ts time.Time,
	ivl time.Duration,
	svcName string,
	span spanAggregationKey,
	count, overflowCount int,
) *modelpb.APMEvent {
	var metricsetSamples []*modelpb.MetricsetSample
	if overflowCount > 0 {
		metricsetSamples = []*modelpb.MetricsetSample{
			{
				Name:  "service_destination.aggregation.overflow_count",
				Value: float64(overflowCount),
			},
		}
	}
	var target *modelpb.ServiceTarget
	if span.TargetName != "" {
		target = &modelpb.ServiceTarget{
			Name: span.TargetName,
		}
	}
	return &modelpb.APMEvent{
		Timestamp: modelpb.FromTime(ts),
		Event:     &modelpb.Event{Received: modelpb.FromTime(time.Now())},
		Metricset: &modelpb.Metricset{
			Name:     "service_destination",
			Interval: formatDuration(ivl),
			Samples:  metricsetSamples,
			DocCount: uint64(count),
		},
		Service: &modelpb.Service{
			Name:   svcName,
			Target: target,
		},
		Span: &modelpb.Span{
			Name: span.SpanName,
			DestinationService: &modelpb.DestinationService{
				Resource: span.Resource,
				ResponseTime: &modelpb.AggregatedDuration{
					// test code generates 1 count for 1 ns
					Count: uint64(count),
					Sum:   uint64(time.Duration(count)),
				},
			},
		},
	}
}

func getTestGlobalLabelsStr(t *testing.T, s string) string {
	t.Helper()
	var gl globalLabels
	gl.Labels = make(modelpb.Labels)
	gl.Labels["test"] = &modelpb.LabelValue{Value: s}
	gls, err := gl.MarshalString()
	if err != nil {
		t.Fatal(err)
	}
	return gls
}

func globalLabelsEvent() *modelpb.APMEvent {
	return &modelpb.APMEvent{
		Labels: modelpb.Labels{
			"tag1": &modelpb.LabelValue{
				Value:  "1",
				Values: nil,
				Global: false,
			},
			"tag2": &modelpb.LabelValue{
				Value:  "2",
				Values: nil,
				Global: true,
			},
			"tag3": &modelpb.LabelValue{
				Value:  "",
				Values: []string{"a", "b"},
				Global: false,
			},
			"tag4": &modelpb.LabelValue{
				Value:  "",
				Values: []string{"c", "d"},
				Global: true,
			},
		},
		NumericLabels: modelpb.NumericLabels{
			"tag1": &modelpb.NumericLabelValue{
				Value:  1.1,
				Values: nil,
				Global: false,
			},
			"tag2": &modelpb.NumericLabelValue{
				Value:  2.2,
				Values: nil,
				Global: true,
			},
			"tag3": &modelpb.NumericLabelValue{
				Value:  0,
				Values: []float64{3.3, 4.4},
				Global: false,
			},
			"tag4": &modelpb.NumericLabelValue{
				Value:  0,
				Values: []float64{5.5, 6.6},
				Global: true,
			},
		},
	}
}

func TestMarshalEventGlobalLabels(t *testing.T) {
	e := globalLabelsEvent()
	b, err := marshalEventGlobalLabels(e)
	require.NoError(t, err)
	gl := globalLabels{}
	err = gl.UnmarshalBinary(b)
	require.NoError(t, err)
	assert.Equal(t, modelpb.Labels{
		"tag2": &modelpb.LabelValue{
			Value:  "2",
			Values: nil,
			Global: true,
		},
		"tag4": &modelpb.LabelValue{
			Value:  "",
			Values: []string{"c", "d"},
			Global: true,
		},
	}, gl.Labels)
	assert.Equal(t, modelpb.NumericLabels{
		"tag2": &modelpb.NumericLabelValue{
			Value:  2.2,
			Values: nil,
			Global: true,
		},
		"tag4": &modelpb.NumericLabelValue{
			Value:  0,
			Values: []float64{5.5, 6.6},
			Global: true,
		},
	}, gl.NumericLabels)
}

func TestMarshalEventGlobalLabelsRace(t *testing.T) {
	const N = 1000
	wg := sync.WaitGroup{}
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			e := globalLabelsEvent()
			b, err := marshalEventGlobalLabels(e)
			require.NoError(t, err)
			gl := globalLabels{}
			err = gl.UnmarshalBinary(b)
			require.NoError(t, err)
			b, err = gl.MarshalBinary()
			require.NoError(t, err)
			err = gl.UnmarshalBinary(b)
			require.NoError(t, err)
			wg.Done()
		}()
	}
	wg.Wait()
}
