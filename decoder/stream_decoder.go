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

// NewNDJSONStreamReader returns a NDJSONStreamReader which reads
// ND-JSON lines from r, with a maximum line length of maxLineLength.
func NewNDJSONStreamReader(r io.Reader, maxLineLength int) *NDJSONStreamReader {
	var sr NDJSONStreamReader
	sr.bufioReader = bufio.NewReaderSize(r, maxLineLength)
	sr.lineReader = NewLineReader(sr.bufioReader, maxLineLength)
	sr.resetDecoder()
	return &sr
}

// NDJSONStreamReader reads and decodes a stream of ND-JSON lines from an io.Reader.
type NDJSONStreamReader struct {
	bufioReader *bufio.Reader
	lineReader  *LineReader

	isEOF            bool
	latestLine       []byte
	latestLineReader bytes.Reader
	decoder          *json.Decoder
}

// Reset sets sr's underlying io.Reader to r, and resets any reading/decoding state.
func (sr *NDJSONStreamReader) Reset(r io.Reader) {
	sr.bufioReader.Reset(r)
	sr.lineReader.Reset(sr.bufioReader)
	sr.isEOF = false
	sr.latestLine = nil
	sr.latestLineReader.Reset(nil)
}

func (sr *NDJSONStreamReader) resetDecoder() {
	sr.decoder = NewJSONDecoder(&sr.latestLineReader)
}

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	buf, readErr := sr.readLine()
	if len(buf) == 0 || (readErr != nil && !sr.isEOF) {
		return nil, readErr
	}
	decoded := make(map[string]interface{})
	if err := sr.decoder.Decode(&decoded); err != nil {
		sr.resetDecoder() // clear out decoding state
		return nil, JSONDecodeError("data read error: " + err.Error())
	}
	return decoded, readErr // this might be io.EOF
}

func (sr *NDJSONStreamReader) readLine() ([]byte, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	line, readErr := sr.lineReader.ReadLine()
	sr.latestLine = line
	sr.latestLineReader.Reset(sr.latestLine)
	sr.isEOF = readErr == io.EOF
	return line, readErr
}

func (sr *NDJSONStreamReader) IsEOF() bool        { return sr.isEOF }
func (sr *NDJSONStreamReader) LatestLine() []byte { return sr.latestLine }

type JSONDecodeError string

func (s JSONDecodeError) Error() string { return string(s) }
