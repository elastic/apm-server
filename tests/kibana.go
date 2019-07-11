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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"

	"github.com/elastic/beats/libbeat/common"
)

// MockKibanaClient implements the kibana.Client interface for testing purposes
type MockKibanaClient struct {
	code      int
	body      map[string]interface{}
	v         common.Version
	connected bool
}

// Send returns a mock http.Response based on parameters used to init the MockKibanaClient instance
func (c *MockKibanaClient) Send(method, extraPath string, params url.Values,
	headers http.Header, body io.Reader) (*http.Response, error) {
	resp := http.Response{StatusCode: c.code, Body: ioutil.NopCloser(convert.ToReader(c.body))}
	if resp.StatusCode == http.StatusBadGateway {
		return nil, errors.New("testerror")
	}
	return &resp, nil
}

// GetVersion returns a mock version based on parameters used to init the MockKibanaClient instance
func (c *MockKibanaClient) GetVersion() (common.Version, error) {
	return c.v, nil
}

// Connected returns whether or not mock client is connected
func (c *MockKibanaClient) Connected() bool { return c.connected }

// SupportsVersion returns whether or not mock client is compatible with given version
func (c *MockKibanaClient) SupportsVersion(v *common.Version) (bool, error) {
	return v.LessThanOrEqual(true, &c.v), nil
}

// MockKibana provides a fake connection for unit tests
func MockKibana(respCode int, respBody map[string]interface{}, v common.Version, connected bool) kibana.Client {
	return &MockKibanaClient{code: respCode, body: respBody, v: v, connected: connected}
}
