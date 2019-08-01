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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/tests"
)

func TestSmapHandler_RequireAuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := cfgEnabledSmap()
		cfg.SecretToken = "1234"
		rec := requestToSmapHandler(t, cfg)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := cfgEnabledSmap()
		cfg.SecretToken = "1234"
		h, err := sourcemapHandler(cfg, beatertest.NilReporter)
		require.NoError(t, err)
		c, rec := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")
		c.Request.Header.Set(headers.Authorization, "Bearer 1234")
		h(c)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
	})
}

func TestSmapHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("OffRum", func(t *testing.T) {
		rec := requestToSmapHandler(t, DefaultConfig(beatertest.MockBeatVersion()))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
	})

	t.Run("OffSourcemap", func(t *testing.T) {
		cfg := DefaultConfig(beatertest.MockBeatVersion())
		rum := true
		cfg.RumConfig.Enabled = &rum
		cfg.RumConfig.SourceMapping.Enabled = new(bool)
		rec := requestToSmapHandler(t, cfg)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
	})

	t.Run("On", func(t *testing.T) {
		rec := requestToSmapHandler(t, cfgEnabledSmap())

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
	})
}

func TestAssetHandler_LogMiddleware(t *testing.T) {
	//TODO: How to test?
}

func TestAssetHandler_PanicMiddleware(t *testing.T) {
	h, err := sourcemapHandler(DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	rec := &beatertest.WriterPanicOnce{}
	c := &request.Context{}
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h(c)
	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	tests.AssertApproveResult(t, assetApprovalPath(t.Name()), rec.Body.Bytes())
}

func TestAssetHandler_MonitoringMiddleware(t *testing.T) {
	h, err := sourcemapHandler(DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	c, _ := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")

	// send GET request resulting in 403 Forbidden error as RUM is disabled by default
	expected := map[request.ResultID]int{
		request.IDRequestCount:            1,
		request.IDResponseCount:           1,
		request.IDResponseErrorsCount:     1,
		request.IDResponseErrorsForbidden: 1}

	equal, result := beatertest.CompareMonitoringInt(h, c, expected, serverMetrics, IntakeResultIDToMonitoringInt)
	assert.True(t, equal, result)
}

func requestToSmapHandler(t *testing.T, cfg *Config) *httptest.ResponseRecorder {
	h, err := sourcemapHandler(cfg, beatertest.NilReporter)
	require.NoError(t, err)
	c, rec := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")
	h(c)
	return rec
}

func cfgEnabledSmap() *Config {
	cfg := DefaultConfig(beatertest.MockBeatVersion())
	t := true
	cfg.RumConfig.Enabled = &t
	return cfg
}

func assetApprovalPath(f string) string { return "test_integration/asset/" + f }
