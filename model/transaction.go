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
	"github.com/elastic/beats/v7/libbeat/common"
)

const (
	TracesDataset = "apm"
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

	// Kind holds the span kind: "CLIENT", "SERVER", "PRODUCER", "CONSUMER" and "INTERNAL".
	Kind string

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

	// BreakdownCount holds transaction breakdown count, for
	// breakdown metrics.
	BreakdownCount int

	Marks          TransactionMarks
	Message        *Message
	SpanCount      SpanCount
	Custom         common.MapStr
	UserExperience *UserExperience

	// DroppedSpanStats holds a list of the spans that were dropped by an
	// agent; not indexed.
	DroppedSpansStats []DroppedSpanStats

	// RepresentativeCount holds the approximate number of
	// transactions that this transaction represents for aggregation.
	//
	// This may be used for scaling metrics; it is not indexed.
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

func (e *Transaction) setFields(fields *mapStr, apmEvent *APMEvent) {
	var transaction mapStr
	if apmEvent.Processor == TransactionProcessor {
		// TODO(axw) set `event.duration` in 8.0, and remove this field.
		// See https://github.com/elastic/apm-server/issues/5999
		transaction.set("duration", common.MapStr{"us": int(apmEvent.Event.Duration.Microseconds())})
	}
	transaction.maybeSetString("id", e.ID)
	transaction.maybeSetString("type", e.Type)
	transaction.maybeSetString("span.kind", e.Kind)
	transaction.maybeSetMapStr("duration.histogram", e.DurationHistogram.fields())
	transaction.maybeSetString("name", e.Name)
	transaction.maybeSetString("result", e.Result)
	transaction.maybeSetMapStr("marks", e.Marks.fields())
	transaction.maybeSetMapStr("custom", customFields(e.Custom))
	transaction.maybeSetMapStr("message", e.Message.Fields())
	transaction.maybeSetMapStr("experience", e.UserExperience.Fields())
	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}
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
	if e.BreakdownCount > 0 {
		transaction.set("breakdown.count", e.BreakdownCount)
	}
	var dss []common.MapStr
	for _, v := range e.DroppedSpansStats {
		dss = append(dss, v.fields())
	}
	if len(dss) > 0 {
		transaction.set("dropped_spans_stats", dss)
	}
	fields.maybeSetMapStr("transaction", common.MapStr(transaction))
}

type TransactionMarks map[string]TransactionMark

func (m TransactionMarks) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(mapStr, len(m))
	for k, v := range m {
		out.maybeSetMapStr(sanitizeLabelKey(k), v.fields())
	}
	return common.MapStr(out)
}

type TransactionMark map[string]float64

func (m TransactionMark) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(common.MapStr, len(m))
	for k, v := range m {
		out[sanitizeLabelKey(k)] = common.Float(v)
	}
	return out
}

type DroppedSpanStats struct {
	DestinationServiceResource string
	Outcome                    string
	Duration                   AggregatedDuration
}

func (stat DroppedSpanStats) fields() common.MapStr {
	var out mapStr
	out.maybeSetString("destination_service_resource",
		stat.DestinationServiceResource,
	)
	out.maybeSetString("outcome", stat.Outcome)
	out.maybeSetMapStr("duration", stat.Duration.fields())
	return common.MapStr(out)
}
