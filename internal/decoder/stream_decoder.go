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
	"io"

	jsoniter "github.com/json-iterator/go"
)

// NewNDJSONStreamDecoder returns a new NDJSONStreamDecoder which decodes
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
	latestError      error
	latestLine       []byte
	latestLineReader bytes.Reader
	decoder          *jsoniter.Decoder
}

// Reset sets sr's underlying io.Reader to r, and resets any reading/decoding state.
func (dec *NDJSONStreamDecoder) Reset(r io.Reader) {
	dec.bufioReader.Reset(r)
	dec.lineReader.Reset(dec.bufioReader)
	dec.isEOF = false
	dec.latestLine = nil
	dec.resetLatestLineReader()
}

func (dec *NDJSONStreamDecoder) resetDecoder() {
	dec.decoder = json.NewDecoder(&dec.latestLineReader)
	dec.decoder.UseNumber()
}

// Decode decodes the next line into v.
func (dec *NDJSONStreamDecoder) Decode(v interface{}) error {
	defer dec.resetLatestLineReader()
	if dec.latestLineReader.Size() == 0 {
		_, _ = dec.ReadAhead() // error checked below
	}
	if len(dec.latestLine) == 0 || (dec.latestError != nil && !dec.isEOF) {
		return dec.latestError
	}
	if err := dec.decoder.Decode(v); err != nil {
		dec.resetDecoder() // clear out decoding state
		return JSONDecodeError("data read error: " + err.Error())
	}
	return dec.latestError // this might be io.EOF
}

// ReadAhead reads the next NDJSON line, buffering it for a subsequent call to Decode.
func (dec *NDJSONStreamDecoder) ReadAhead() ([]byte, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	line, readErr := dec.lineReader.ReadLine()
	dec.latestLine = line
	dec.latestLineReader.Reset(dec.latestLine)
	dec.latestError = readErr
	dec.isEOF = readErr == io.EOF
	return line, readErr
}

func (dec *NDJSONStreamDecoder) resetLatestLineReader() {
	dec.latestLineReader.Reset(nil)
	dec.latestError = nil
}

// IsEOF signals whether the underlying reader reached the end
func (dec *NDJSONStreamDecoder) IsEOF() bool { return dec.isEOF }

// LatestLine returns the latest line read as []byte
func (dec *NDJSONStreamDecoder) LatestLine() []byte { return dec.latestLine }

// JSONDecodeError is a custom error that can occur during JSON decoding
type JSONDecodeError string

func (s JSONDecodeError) Error() string { return string(s) }
