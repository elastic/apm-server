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

package modeldecodertest

import (
	"bufio"
	"bytes"
	"io"

	jsoniter "github.com/json-iterator/go"

	"github.com/elastic/apm-server/decoder"
)

var json = jsoniter.ConfigFastest

// TODO(simitt):
// copied decoder from decoder package
// remove this file again when the ReadAhaed functionality is introduced to
// the decoder.StreamDecoder

func newNDJSONStreamDecoder(r io.Reader, maxLineLength int) *ndjsonStreamDecoder {
	var dec ndjsonStreamDecoder
	dec.bufioReader = bufio.NewReaderSize(r, maxLineLength)
	dec.lineReader = decoder.NewLineReader(dec.bufioReader, maxLineLength)
	dec.resetDecoder()
	return &dec
}

type ndjsonStreamDecoder struct {
	bufioReader *bufio.Reader
	lineReader  *decoder.LineReader

	isEOF            bool
	latestError      error
	latestLine       []byte
	latestLineReader bytes.Reader
	decoder          *jsoniter.Decoder
}

func (dec *ndjsonStreamDecoder) resetDecoder() {
	dec.decoder = json.NewDecoder(&dec.latestLineReader)
	dec.decoder.UseNumber()
}

// Decode decodes the next line into v.
func (dec *ndjsonStreamDecoder) decode(v interface{}) error {
	defer dec.resetLatestLineReader()
	if dec.latestLineReader.Size() == 0 {
		dec.readAhead()
	}
	if len(dec.latestLine) == 0 || (dec.latestError != nil && !dec.isEOF) {
		return dec.latestError
	}
	if err := dec.decoder.Decode(v); err != nil {
		dec.resetDecoder() // clear out decoding state
		return jsonDecodeError("data read error: " + err.Error())
	}
	return dec.latestError // this might be io.EOF
}

func (dec *ndjsonStreamDecoder) readAhead() ([]byte, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	line, readErr := dec.lineReader.ReadLine()
	dec.latestLine = line
	dec.latestLineReader.Reset(dec.latestLine)
	dec.latestError = readErr
	dec.isEOF = readErr == io.EOF
	return line, readErr
}

func (dec *ndjsonStreamDecoder) resetLatestLineReader() {
	dec.latestLineReader.Reset(nil)
	dec.latestError = nil
}

type jsonDecodeError string

func (s jsonDecodeError) Error() string { return string(s) }
