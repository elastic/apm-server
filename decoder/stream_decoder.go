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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func NDJSONStreamDecodeCompressedWithLimit(req *http.Request, lineLimit int) (*NDJSONStreamReader, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/x-ndjson") {
		return nil, fmt.Errorf("invalid content type: '%s'", req.Header.Get("Content-Type"))
	}

	reader, err := CompressedRequestReader(req)
	if err != nil {
		return nil, err
	}

	return NewNDJSONStreamReader(reader, lineLimit), nil
}

func NewNDJSONStreamReader(reader io.Reader, lineLimit int) *NDJSONStreamReader {
	return &NDJSONStreamReader{reader: NewLineReader(reader, lineLimit)}
}

type NDJSONStreamReader struct {
	reader     *LineReader
	isEOF      bool
	latestLine []byte
}

type JSONDecodeError string

func (s JSONDecodeError) Error() string { return string(s) }

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	// readLine can return valid data in `buf` _and_ also an io.EOF
	buf, readErr := sr.reader.ReadLine()
	sr.latestLine = buf

	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}

	sr.isEOF = readErr == io.EOF

	if len(buf) == 0 {
		return nil, readErr
	}
	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := DecodeJSONData(tmpreader)
	if err != nil {
		return nil, JSONDecodeError(err.Error())
	}

	return decoded, readErr // this might be io.EOF
}

func (sr *NDJSONStreamReader) IsEOF() bool        { return sr.isEOF }
func (sr *NDJSONStreamReader) LatestLine() []byte { return sr.latestLine }
