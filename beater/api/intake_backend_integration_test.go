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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/tests"
)

func TestIntakeBackendHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := config.DefaultConfig(beatertest.MockBeatVersion())
		cfg.SecretToken = "1234"
		rec := requestToIntakeBackendHandler(t, cfg)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		tests.AssertApproveResult(t, approvalPathIntakeBackend(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := config.DefaultConfig(beatertest.MockBeatVersion())
		cfg.SecretToken = "1234"
		h, err := backendHandler(cfg, beatertest.NilReporter)
		require.NoError(t, err)
		c, rec := beatertest.DefaultContextWithResponseRecorder()
		c.Request.Header.Set(headers.Authorization, "Bearer 1234")
		h(c)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		tests.AssertApproveResult(t, approvalPathIntakeBackend(t.Name()), rec.Body.Bytes())
	})
}

func TestIntakeBackendHandler_PanicMiddleware(t *testing.T) {
	h, err := backendHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	rec := &beatertest.WriterPanicOnce{}
	c := &request.Context{}
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h(c)
	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	tests.AssertApproveResult(t, approvalPathIntakeBackend(t.Name()), rec.Body.Bytes())
}

func TestIntakeBackendHandler_MonitoringMiddleware(t *testing.T) {

	h, err := backendHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	c, _ := beatertest.ContextWithResponseRecorder(http.MethodGet, "/")
	// send GET request resulting in 405 MethodNotAllowed error
	expected := map[request.ResultID]int{
		request.IDRequestCount:                   1,
		request.IDResponseCount:                  1,
		request.IDResponseErrorsCount:            1,
		request.IDResponseErrorsMethodNotAllowed: 1}

	equal, result := beatertest.CompareMonitoringInt(h, c, expected, intake.MonitoringRegistry, intake.ResultIDToMonitoringInt)
	assert.True(t, equal, result)
}

func requestToIntakeBackendHandler(t *testing.T, cfg *config.Config) *httptest.ResponseRecorder {
	h, err := backendHandler(cfg, beatertest.NilReporter)
	require.NoError(t, err)
	c, rec := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")
	h(c)
	return rec
}

func approvalPathIntakeBackend(f string) string {
	return "intake/test_approved/integration/backend/" + f
}
