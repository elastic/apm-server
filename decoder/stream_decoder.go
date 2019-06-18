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
	"bytes"
	"io"
	"io/ioutil"
)

func NewNDJSONStreamReader(reader *LineReader) *NDJSONStreamReader {
	return &NDJSONStreamReader{reader: reader}
}

type NDJSONStreamReader struct {
	reader     *LineReader
	isEOF      bool
	latestLine []byte
}

type JSONDecodeError string

func (s JSONDecodeError) Error() string { return string(s) }

// Read consumes the reader returning its content, an error if any, and
// a boolean indicating if the error is terminal (ie. another Read call can be performed)
func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error, bool) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	buf, readErr := sr.reader.ReadLine()
	sr.latestLine = buf

	if readErr == ErrLineTooLong {
		return nil, readErr, false
	}

	if readErr != nil && readErr != io.EOF {
		return nil, readErr, true
	}

	sr.isEOF = readErr == io.EOF

	if len(buf) == 0 {
		return nil, readErr, true
	}
	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := DecodeJSONData(tmpreader)
	if err != nil {
		return nil, JSONDecodeError(err.Error()), false
	}

	return decoded, readErr, readErr != nil // this might be io.EOF
}

func (sr *NDJSONStreamReader) IsEOF() bool        { return sr.isEOF }
func (sr *NDJSONStreamReader) LatestLine() []byte { return sr.latestLine }
