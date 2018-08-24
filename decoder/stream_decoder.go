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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func NDJSONStreamDecodeCompressedWithLimit(req *http.Request, maxSize int64) (*NDJSONStreamReader, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/x-ndjson") {
		return nil, fmt.Errorf("invalid content type: '%s'", req.Header.Get("Content-Type"))
	}

	reader, err := CompressedRequestReader(maxSize)(req)
	if err != nil {
		return nil, err
	}

	return NewNDJSONStreamReader(reader), nil
}

func NewNDJSONStreamReader(reader io.Reader) *NDJSONStreamReader {
	return &NDJSONStreamReader{bufio.NewReader(reader), false, nil}
}

type NDJSONStreamReader struct {
	stream     *bufio.Reader
	isEOF      bool
	latestLine []byte
}

type JSONDecodeError string
type ReadError string

func (s JSONDecodeError) Error() string { return string(s) }
func (s ReadError) Error() string       { return string(s) }

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	buf, readErr := sr.stream.ReadBytes('\n')
	if readErr != nil && readErr != io.EOF {
		return nil, ReadError(readErr.Error())
	}

	sr.isEOF = readErr == io.EOF

	if len(buf) == 0 {
		return nil, readErr
	}

	sr.latestLine = buf

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := DecodeJSONData(tmpreader)
	if err != nil {
		return nil, JSONDecodeError(err.Error())
	}

	return decoded, readErr // this might be io.EOF
}

func (sr *NDJSONStreamReader) IsEOF() bool      { return sr.isEOF }
func (sr *NDJSONStreamReader) LastLine() []byte { return sr.latestLine }
