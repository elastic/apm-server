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

package apm

import (
	"encoding/hex"
	"errors"
)

var (
	errZeroTraceID = errors.New("zero trace-id is invalid")
	errZeroSpanID  = errors.New("zero span-id is invalid")
)

const (
	traceOptionsRecordedFlag = 0x01
)

// TraceContext holds trace context for an incoming or outgoing request.
type TraceContext struct {
	// Trace identifies the trace forest.
	Trace TraceID

	// Span identifies a span: the parent span if this context
	// corresponds to an incoming request, or the current span
	// if this is an outgoing request.
	Span SpanID

	// Options holds the trace options propagated by the parent.
	Options TraceOptions
}

// TraceID identifies a trace forest.
type TraceID [16]byte

// Validate validates the trace ID.
// This will return non-nil for a zero trace ID.
func (id TraceID) Validate() error {
	if id.isZero() {
		return errZeroTraceID
	}
	return nil
}

func (id TraceID) isZero() bool {
	return id == (TraceID{})
}

// String returns id encoded as hex.
func (id TraceID) String() string {
	text, _ := id.MarshalText()
	return string(text)
}

// MarshalText returns id encoded as hex, satisfying encoding.TextMarshaler.
func (id TraceID) MarshalText() ([]byte, error) {
	text := make([]byte, hex.EncodedLen(len(id)))
	hex.Encode(text, id[:])
	return text, nil
}

// SpanID identifies a span within a trace.
type SpanID [8]byte

// Validate validates the span ID.
// This will return non-nil for a zero span ID.
func (id SpanID) Validate() error {
	if id.isZero() {
		return errZeroSpanID
	}
	return nil
}

func (id SpanID) isZero() bool {
	return id == SpanID{}
}

// String returns id encoded as hex.
func (id SpanID) String() string {
	text, _ := id.MarshalText()
	return string(text)
}

// MarshalText returns id encoded as hex, satisfying encoding.TextMarshaler.
func (id SpanID) MarshalText() ([]byte, error) {
	text := make([]byte, hex.EncodedLen(len(id)))
	hex.Encode(text, id[:])
	return text, nil
}

// TraceOptions describes the options for a trace.
type TraceOptions uint8

// Recorded reports whether or not the transaction/span may have been (or may be) recorded.
func (o TraceOptions) Recorded() bool {
	return (o & traceOptionsRecordedFlag) == traceOptionsRecordedFlag
}

// WithRecorded changes the "recorded" flag, and returns the new options
// without modifying the original value.
func (o TraceOptions) WithRecorded(recorded bool) TraceOptions {
	if recorded {
		return o | traceOptionsRecordedFlag
	}
	return o & (0xFF ^ traceOptionsRecordedFlag)
}
