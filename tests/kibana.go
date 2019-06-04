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

package tests

import (
	"io/ioutil"
	"net/http"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/kibana"
)

type rt struct {
	resp *http.Response
}

// RoundTrip implements the Round Tripper interface
func (rt rt) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.resp, nil
}

// MockKibana provides a fake connection for unit tests
func MockKibana(respCode int, respBody map[string]interface{}) *kibana.Client {
	resp := http.Response{StatusCode: respCode, Body: ioutil.NopCloser(convert.ToReader(respBody))}
	return &kibana.Client{
		Connection: kibana.Connection{
			Http: &http.Client{
				Transport: rt{resp: &resp},
			},
			Version: common.Version{Major: 10, Minor: 0},
		},
	}
}
