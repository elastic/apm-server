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

package beater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

func TestMonitoringHandler(t *testing.T) {
	requestCounter := mockMonitoringFn(request.IDRequestCount)
	responseCounter := mockMonitoringFn(request.IDResponseCount)
	responseErrors := mockMonitoringFn(request.IDResponseErrorsCount)
	responseSuccesses := mockMonitoringFn(request.IDResponseValidCount)

	t.Run("Error", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/assets/v1/sourcemaps/")
		withMiddleware(mockHandler403, monitoringHandler(mockMonitoringFn))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseErrors: 1,
			mockMonitoringFn(request.IDResponseErrorsForbidden): 1})
	})
	t.Run("Accepted", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/assets/v1/sourcemaps")
		withMiddleware(mockHandler202, monitoringHandler(mockMonitoringFn))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1,
			mockMonitoringFn(request.IDResponseValidAccepted): 1})
	})
	t.Run("Idle", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/assets/v1/sourcemaps")

		withMiddleware(mockHandlerIdle, monitoringHandler(mockMonitoringFn))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1})
	})
}

func testCounter(t *testing.T, ctrs map[*monitoring.Int]int64) {
	for idx, ct := range testGetCounter() {
		actual := ct.Get()
		expected := int64(0)
		if val, included := ctrs[ct]; included {
			expected = val
		}
		assert.Equal(t, expected, actual, fmt.Sprintf("Idx: %d", idx))
	}
}

func setupContext(path string) *request.Context {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	c := &request.Context{}
	c.Reset(w, r)
	return c
}

func mockHandler403(c *request.Context) {
	c.MonitoringID = request.IDResponseErrorsForbidden
	c.WriteHeader(http.StatusForbidden)
}

func mockHandler202(c *request.Context) {
	c.MonitoringID = request.IDResponseValidAccepted
	c.WriteHeader(http.StatusAccepted)
}

func mockHandlerIdle(c *request.Context) {}

func testResetCounter() {
	for _, ct := range testGetCounter() {
		ct.Set(0)
	}
}

func testGetCounter() []*monitoring.Int {
	return []*monitoring.Int{
		mockMonitoringFn(request.IDRequestCount),
		mockMonitoringFn(request.IDResponseCount),
		mockMonitoringFn(request.IDResponseErrorsCount),
		mockMonitoringFn(request.IDResponseValidCount),
		mockMonitoringFn(request.IDResponseValidOK),
		mockMonitoringFn(request.IDResponseValidAccepted),
		mockMonitoringFn(request.IDResponseErrorsInternal),
		mockMonitoringFn(request.IDResponseErrorsForbidden),
		mockMonitoringFn(request.IDResponseErrorsRequestTooLarge),
		mockMonitoringFn(request.IDResponseErrorsDecode),
		mockMonitoringFn(request.IDResponseErrorsValidate),
		mockMonitoringFn(request.IDResponseErrorsRateLimit),
		mockMonitoringFn(request.IDResponseErrorsMethodNotAllowed),
		mockMonitoringFn(request.IDResponseErrorsFullQueue),
		mockMonitoringFn(request.IDResponseErrorsShuttingDown),
		mockMonitoringFn(request.IDResponseErrorsUnauthorized),
	}
}

var (
	mockMonitoringRegistry = monitoring.Default.NewRegistry("mock.monitoring", monitoring.PublishExpvar)
	mockMonitoringMap      = map[request.ResultID]*monitoring.Int{}
	mockCounterFn          = func(s string) *monitoring.Int {
		return monitoring.NewInt(mockMonitoringRegistry, s)
	}
)

func mockMonitoringFn(name request.ResultID) *monitoring.Int {
	if i, ok := mockMonitoringMap[name]; ok {
		return i
	}
	ct := mockCounterFn(string(name))
	mockMonitoringMap[name] = ct
	return ct
}
