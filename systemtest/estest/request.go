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

package estest

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type Request interface {
	Do(ctx context.Context, transport esapi.Transport) (*esapi.Response, error)
}

// bodyRepeater wraps an esapi.Transport, copying the request body on the first
// call to Perform as needed, and replacing the request body on subsequent calls.
type bodyRepeater struct {
	t       esapi.Transport
	getBody func() (io.ReadCloser, error)
}

func (br *bodyRepeater) Perform(req *http.Request) (*http.Response, error) {
	if req.Body == nil {
		return br.t.Perform(req)
	}
	if br.getBody == nil {
		// First call to Perform: set br.getBody to req.GetBody
		// (if non-nil), or otherwise by replacing req.Body with
		// a TeeReader that reads into a buffer, and setting
		// getBody to a function that returns that buffer on
		// subsequent calls.
		br.getBody = req.GetBody
		if br.getBody == nil {
			// no GetBody, gotta copy it ourselves
			var buf bytes.Buffer
			req.Body = readCloser{
				Reader: io.TeeReader(req.Body, &buf),
				Closer: req.Body,
			}
			br.getBody = func() (io.ReadCloser, error) {
				r := bytes.NewReader(buf.Bytes())
				return ioutil.NopCloser(r), nil
			}
		}
	} else {
		body, err := br.getBody()
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = body
	}
	return br.t.Perform(req)
}

type readCloser struct {
	io.Reader
	io.Closer
}
