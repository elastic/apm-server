// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package model

import (
	"github.com/elastic/elastic-agent-libs/mapstr"
)

var (
	// TransactionProcessor is the Processor value that should be assigned to transaction events.
	TransactionProcessor = Processor{Name: "transaction", Event: "transaction"}
)

// Transaction holds values for transaction.* fields. This may be used in
// transaction, span, and error events (i.e. transaction.id), as well as
// internal metrics such as breakdowns (i.e. including transaction.name).
type Transaction struct {
	ID string

	// Name holds the transaction name: "GET /foo", etc.
	Name string

	// Type holds the transaction type: "request", "message", etc.
	Type string

	// Result holds the transaction result: "HTTP 2xx", "OK", "Error", etc.
	Result string

	// Sampled holds the transaction's sampling decision.
	//
	// If Sampled is false, then it will be omitted from the output event.
	Sampled bool

	// DurationHistogram holds a transaction duration histogram,
	// with bucket values measured in microseconds, for transaction
	// duration metrics.
	DurationHistogram Histogram

	// DurationSummary holds an aggregated transaction duration summary,
	// for service metrics. The DurationSummary.Sum field has microsecond
	// resolution.
	//
	// NOTE(axw) this is used only for service metrics, which are in technical
	// preview. Do not use this field without discussion, as the field mapping
	// is subject to removal.
	DurationSummary SummaryMetric

	// FailureCount holds an aggregated count of transactions with the
	// outcome "failure". If FailureCount is zero, it will be omitted from
	// the output event.
	//
	// NOTE(axw) this is used only for service metrics, which are in technical
	// preview. Do not use this field without discussion, as the field mapping
	// is subject to removal.
	FailureCount int

	// SuccessCount holds an aggregated count of transactions with different
	// outcomes. A "failure" adds to the Count. A "success" adds to both the
	// Count and the Sum. An "unknown" has no effect. If Count is zero, it
	// will be omitted from the output event.
	SuccessCount SummaryMetric

	Marks          TransactionMarks
	Message        *Message
	SpanCount      SpanCount
	Custom         mapstr.M
	UserExperience *UserExperience

	// DroppedSpanStats holds a list of the spans that were dropped by an
	// agent; not indexed.
	DroppedSpansStats []DroppedSpanStats

	// RepresentativeCount holds the approximate number of
	// transactions that this transaction represents for aggregation.
	// This is used for scaling metrics.
	RepresentativeCount float64

	// Root indicates whether or not the transaction is the trace root.
	//
	// If Root is false, it will be omitted from the output event.
	Root bool
}

type SpanCount struct {
	Dropped *int
	Started *int
}

func (e *Transaction) fields() mapstr.M {
	var transaction mapStr
	transaction.maybeSetString("id", e.ID)
	transaction.maybeSetString("type", e.Type)
	transaction.maybeSetMapStr("duration.histogram", e.DurationHistogram.fields())
	if e.DurationSummary.Count != 0 {
		transaction.maybeSetMapStr("duration.summary", e.DurationSummary.fields())
	}
	if e.FailureCount != 0 {
		transaction.set("failure_count", e.FailureCount)
	}
	if e.SuccessCount.Count != 0 {
		transaction.maybeSetMapStr("success_count", e.SuccessCount.fields())
	}
	transaction.maybeSetString("name", e.Name)
	transaction.maybeSetString("result", e.Result)
	transaction.maybeSetMapStr("marks", e.Marks.fields())
	transaction.maybeSetMapStr("custom", customFields(e.Custom))
	transaction.maybeSetMapStr("message", e.Message.Fields())
	transaction.maybeSetMapStr("experience", e.UserExperience.Fields())
	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := mapstr.M{}
		if e.SpanCount.Dropped != nil {
			spanCount["dropped"] = *e.SpanCount.Dropped
		}
		if e.SpanCount.Started != nil {
			spanCount["started"] = *e.SpanCount.Started
		}
		transaction.set("span_count", spanCount)
	}
	if e.Sampled {
		transaction.set("sampled", e.Sampled)
	}
	if e.Root {
		transaction.set("root", e.Root)
	}
	if e.RepresentativeCount > 0 {
		transaction.set("representative_count", e.RepresentativeCount)
	}
	var dss []mapstr.M
	for _, v := range e.DroppedSpansStats {
		dss = append(dss, v.fields())
	}
	if len(dss) > 0 {
		transaction.set("dropped_spans_stats", dss)
	}
	return mapstr.M(transaction)
}

type TransactionMarks map[string]TransactionMark

func (m TransactionMarks) fields() mapstr.M {
	if len(m) == 0 {
		return nil
	}
	out := make(mapStr, len(m))
	for k, v := range m {
		out.maybeSetMapStr(sanitizeLabelKey(k), v.fields())
	}
	return mapstr.M(out)
}

type TransactionMark map[string]float64

func (m TransactionMark) fields() mapstr.M {
	if len(m) == 0 {
		return nil
	}
	out := make(mapstr.M, len(m))
	for k, v := range m {
		out[sanitizeLabelKey(k)] = v
	}
	return out
}

type DroppedSpanStats struct {
	DestinationServiceResource string
	ServiceTargetType          string
	ServiceTargetName          string
	Outcome                    string
	Duration                   AggregatedDuration
}

func (stat DroppedSpanStats) fields() mapstr.M {
	var out mapStr
	out.maybeSetString("destination_service_resource",
		stat.DestinationServiceResource,
	)
	out.maybeSetString("service_target_type", stat.ServiceTargetType)
	out.maybeSetString("service_target_name", stat.ServiceTargetName)
	out.maybeSetString("outcome", stat.Outcome)
	out.maybeSetMapStr("duration", stat.Duration.fields())
	return mapstr.M(out)
}
