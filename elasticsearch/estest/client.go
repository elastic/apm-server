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
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
)

// Transport can be used to pass to test Elasticsearch Client for more control over client behavior
type Transport struct {
	roundTripFn func(req *http.Request) (*http.Response, error)
	executed    int
}

// RoundTrip implements http.RoundTripper interface
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.executed++
	return t.roundTripFn(req)
}

// NewTransport creates test transport instance returning status code and body according to input parameters when called.
func NewTransport(t *testing.T, statusCode int, esBody map[string]interface{}) *Transport {

	return &Transport{
		roundTripFn: func(_ *http.Request) (*http.Response, error) {
			if statusCode == -1 {
				return nil, errors.New("client error")
			}
			var body io.ReadCloser
			if esBody == nil {
				body = ioutil.NopCloser(bytes.NewReader([]byte{}))
			} else {
				resp, err := json.Marshal(esBody)
				require.NoError(t, err)
				body = ioutil.NopCloser(bytes.NewReader(resp))
			}
			return &http.Response{
				StatusCode: statusCode,
				Body:       body,
				Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
			}, nil
		},
	}
}

// NewElasticsearchClient creates ES client using the given transport instance
func NewElasticsearchClient(transport *Transport) (elasticsearch.Client, error) {
	return elasticsearch.NewVersionedClient("", "", "", []string{}, nil, transport, 3, elasticsearch.DefaultBackoff)
}
