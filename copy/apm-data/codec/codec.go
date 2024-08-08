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

// Package codec provides the interface definitions for encoding types into
// their byte slice representation, and decoding a byte slice into types.
package codec

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// Encoder encodes a type into its byte slice representation.
type Encoder interface {
	// Encode encodes a type into its byte slice representation.
	Encode(in any) ([]byte, error)
}

// Decoder decodes a byte slice representation into its Go type.
type Decoder interface {
	// Decode decodes a byte slice representation into its Go type.
	Decode(in []byte, out any) error
}

// Codec is a composite of Encoder and Decoder.
type Codec interface {
	Encoder
	Decoder
}

// RecordEncodedBytes decorates an encoder with a metric that records the bytes
// that have been encoded.
func RecordEncodedBytes(e Encoder, m metric.Int64Counter) Encoder {
	return metricsRecorder{encoder: e, encoded: m}
}

// RecordDecodedBytes decorates a decoder with a metric that records the bytes
// that have been decoded.
func RecordDecodedBytes(d Decoder, m metric.Int64Counter) Decoder {
	return metricsRecorder{decoder: d, decoded: m}
}

type metricsRecorder struct {
	encoder Encoder
	decoder Decoder

	encoded metric.Int64Counter
	decoded metric.Int64Counter
}

// Encode encodes a type into its byte slice representation.
func (m metricsRecorder) Encode(in any) ([]byte, error) {
	b, err := m.encoder.Encode(in)
	if m.encoded != nil {
		m.encoded.Add(context.Background(), int64(len(b)))
	}
	return b, err
}

// Decode decodes a byte slice representation into its Go type.
func (m metricsRecorder) Decode(in []byte, out any) error {
	if m.decoded != nil {
		m.decoded.Add(context.Background(), int64(len(in)))
	}
	return m.decoder.Decode(in, out)
}
