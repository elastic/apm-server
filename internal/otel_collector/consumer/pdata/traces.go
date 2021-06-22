// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pdata

import (
	"go.opentelemetry.io/collector/internal"
	otlpcollectortrace "go.opentelemetry.io/collector/internal/data/protogen/collector/trace/v1"
	otlptrace "go.opentelemetry.io/collector/internal/data/protogen/trace/v1"
)

// This file defines in-memory data structures to represent traces (spans).

// Traces is the top-level struct that is propagated through the traces pipeline.
type Traces struct {
	orig *otlpcollectortrace.ExportTraceServiceRequest
}

// NewTraces creates a new Traces.
func NewTraces() Traces {
	return Traces{orig: &otlpcollectortrace.ExportTraceServiceRequest{}}
}

// TracesFromInternalRep creates Traces from the internal representation.
// Should not be used outside this module.
func TracesFromInternalRep(wrapper internal.TracesWrapper) Traces {
	return Traces{orig: internal.TracesToOtlp(wrapper)}
}

// TracesFromOtlpProtoBytes converts OTLP Collector ExportTraceServiceRequest
// ProtoBuf bytes to the internal Traces.
//
// Returns an invalid Traces instance if error is not nil.
func TracesFromOtlpProtoBytes(data []byte) (Traces, error) {
	req := otlpcollectortrace.ExportTraceServiceRequest{}
	if err := req.Unmarshal(data); err != nil {
		return Traces{}, err
	}
	internal.TracesCompatibilityChanges(&req)
	return Traces{orig: &req}, nil
}

// InternalRep returns internal representation of the Traces.
// Should not be used outside this module.
func (td Traces) InternalRep() internal.TracesWrapper {
	return internal.TracesFromOtlp(td.orig)
}

// ToOtlpProtoBytes converts this Traces to the OTLP Collector ExportTraceServiceRequest
// ProtoBuf bytes.
//
// Returns an nil byte-array if error is not nil.
func (td Traces) ToOtlpProtoBytes() ([]byte, error) {
	return td.orig.Marshal()
}

// Clone returns a copy of Traces.
func (td Traces) Clone() Traces {
	cloneTd := NewTraces()
	td.ResourceSpans().CopyTo(cloneTd.ResourceSpans())
	return cloneTd
}

// SpanCount calculates the total number of spans.
func (td Traces) SpanCount() int {
	spanCount := 0
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			spanCount += ilss.At(j).Spans().Len()
		}
	}
	return spanCount
}

// OtlpProtoSize returns the size in bytes of this Traces encoded as OTLP Collector
// ExportTraceServiceRequest ProtoBuf bytes.
func (td Traces) OtlpProtoSize() int {
	return td.orig.Size()
}

// ResourceSpans returns the ResourceSpansSlice associated with this Metrics.
func (td Traces) ResourceSpans() ResourceSpansSlice {
	return newResourceSpansSlice(&td.orig.ResourceSpans)
}

// TraceState in w3c-trace-context format: https://www.w3.org/TR/trace-context/#tracestate-header
type TraceState string

const (
	// TraceStateEmpty represents the empty TraceState.
	TraceStateEmpty TraceState = ""
)

// SpanKind is the type of span. Can be used to specify additional relationships between spans
// in addition to a parent/child relationship.
type SpanKind int32

// String returns the string representation of the SpanKind.
func (sk SpanKind) String() string { return otlptrace.Span_SpanKind(sk).String() }

const (
	// SpanKindUnspecified represents that the SpanKind is unspecified, it MUST NOT be used.
	SpanKindUnspecified = SpanKind(0)
	// SpanKindInternal indicates that the span represents an internal operation within an application,
	// as opposed to an operation happening at the boundaries. Default value.
	SpanKindInternal = SpanKind(otlptrace.Span_SPAN_KIND_INTERNAL)
	// SpanKindServer indicates that the span covers server-side handling of an RPC or other
	// remote network request.
	SpanKindServer = SpanKind(otlptrace.Span_SPAN_KIND_SERVER)
	// SpanKindClient indicates that the span describes a request to some remote service.
	SpanKindClient = SpanKind(otlptrace.Span_SPAN_KIND_CLIENT)
	// SpanKindProducer indicates that the span describes a producer sending a message to a broker.
	// Unlike CLIENT and SERVER, there is often no direct critical path latency relationship
	// between producer and consumer spans.
	// A PRODUCER span ends when the message was accepted by the broker while the logical processing of
	// the message might span a much longer time.
	SpanKindProducer = SpanKind(otlptrace.Span_SPAN_KIND_PRODUCER)
	// SpanKindConsumer indicates that the span describes consumer receiving a message from a broker.
	// Like the PRODUCER kind, there is often no direct critical path latency relationship between
	// producer and consumer spans.
	SpanKindConsumer = SpanKind(otlptrace.Span_SPAN_KIND_CONSUMER)
)

// StatusCode mirrors the codes defined at
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
type StatusCode int32

const (
	StatusCodeUnset = StatusCode(otlptrace.Status_STATUS_CODE_UNSET)
	StatusCodeOk    = StatusCode(otlptrace.Status_STATUS_CODE_OK)
	StatusCodeError = StatusCode(otlptrace.Status_STATUS_CODE_ERROR)
)

func (sc StatusCode) String() string { return otlptrace.Status_StatusCode(sc).String() }

// SetCode replaces the code associated with this SpanStatus.
func (ms SpanStatus) SetCode(v StatusCode) {
	ms.orig.Code = otlptrace.Status_StatusCode(v)

	// According to OTLP spec we also need to set the deprecated_code field as we are a new sender:
	// See https://github.com/open-telemetry/opentelemetry-proto/blob/59c488bfb8fb6d0458ad6425758b70259ff4a2bd/opentelemetry/proto/trace/v1/trace.proto#L239
	switch v {
	case StatusCodeUnset, StatusCodeOk:
		ms.orig.DeprecatedCode = otlptrace.Status_DEPRECATED_STATUS_CODE_OK
	case StatusCodeError:
		ms.orig.DeprecatedCode = otlptrace.Status_DEPRECATED_STATUS_CODE_UNKNOWN_ERROR
	}
}
