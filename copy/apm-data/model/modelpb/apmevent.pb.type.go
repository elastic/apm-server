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

package modelpb

type APMEventType int

const (
	UndefinedEventType APMEventType = iota
	MetricEventType
	ErrorEventType
	LogEventType
	SpanEventType
	TransactionEventType

	// MaxEventType holds the highest defined value APMEventType.
	// This can be used for sizing arrays.
	MaxEventType = TransactionEventType
)

// String returns a string representation of the event type:
// "undefined", "metric", "error", "log", "span", or "transaction".
func (t APMEventType) String() string {
	switch t {
	case MetricEventType:
		return "metric"
	case ErrorEventType:
		return "error"
	case LogEventType:
		return "log"
	case SpanEventType:
		return "span"
	case TransactionEventType:
		return "transaction"
	}
	return "undefined"
}

// Type returns the APMEventType for an APMEvent.
//
// The event type is inferred from fields set, in the order:
//
//   - if Metricset is non-nil, MetricEventType is returned
//   - if Error is non-nil, ErrorEventType is returned
//   - if Log is non-nil, or Event.Kind is "event", LogEventType is returned
//   - if Span.Type is non-empty, SpanEventType is returned
//   - if Transaction.Type is non-empty, TransactionEventType is returned
//   - otherwise, UndefinedEventType is returned
func (a *APMEvent) Type() APMEventType {
	switch {
	case a.Metricset != nil:
		return MetricEventType
	case a.Error != nil:
		return ErrorEventType
	case a.Log != nil || a.Event.GetKind() == "event":
		return LogEventType
	case a.Span.GetType() != "":
		return SpanEventType
	case a.Transaction.GetType() != "":
		return TransactionEventType
	}
	return UndefinedEventType
}
