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

package decoder

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

// NewNDJSONStreamDecoder returns a NewNDJSONStreamDecoder which decodes
// ND-JSON lines from r, with a maximum line length of maxLineLength.
func NewNDJSONStreamDecoder(r io.Reader, maxLineLength int) *NDJSONStreamDecoder {
	var dec NDJSONStreamDecoder
	dec.bufioReader = bufio.NewReaderSize(r, maxLineLength)
	dec.lineReader = NewLineReader(dec.bufioReader, maxLineLength)
	dec.resetDecoder()
	return &dec
}

// NDJSONStreamDecoder decodes a stream of ND-JSON lines from an io.Reader.
type NDJSONStreamDecoder struct {
	bufioReader *bufio.Reader
	lineReader  *LineReader

	isEOF            bool
	latestLine       []byte
	latestLineReader bytes.Reader
	decoder          *json.Decoder
}

// Reset sets sr's underlying io.Reader to r, and resets any reading/decoding state.
func (dec *NDJSONStreamDecoder) Reset(r io.Reader) {
	dec.bufioReader.Reset(r)
	dec.lineReader.Reset(dec.bufioReader)
	dec.isEOF = false
	dec.latestLine = nil
	dec.latestLineReader.Reset(nil)
}

func (dec *NDJSONStreamDecoder) resetDecoder() {
	dec.decoder = NewJSONDecoder(&dec.latestLineReader)
}

// Decode decodes the next line into the given interfacedec
func (dec *NDJSONStreamDecoder) Decode(v interface{}) error {
	buf, readErr := dec.readLine()
	if len(buf) == 0 || (readErr != nil && !dec.isEOF) {
		return readErr
	}
	if err := dec.decoder.Decode(v); err != nil {
		dec.resetDecoder() // clear out decoding state
		return JSONDecodeError("data read error: " + err.Error())
	}
	return readErr // this might be io.EOF
}

func (dec *NDJSONStreamDecoder) readLine() ([]byte, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	line, readErr := dec.lineReader.ReadLine()
	dec.latestLine = line
	dec.latestLineReader.Reset(dec.latestLine)
	dec.isEOF = readErr == io.EOF
	return line, readErr
}

// IsEOF signals whether the underlying reader reached the end
func (dec *NDJSONStreamDecoder) IsEOF() bool { return dec.isEOF }

// LatestLine returns the latest line read as []byte
func (dec *NDJSONStreamDecoder) LatestLine() []byte { return dec.latestLine }

// JSONDecodeError is a custom error that can occur during JSON decoding
type JSONDecodeError string

func (s JSONDecodeError) Error() string { return string(s) }
