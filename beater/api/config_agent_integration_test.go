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

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/tests"
)

func TestConfigAgentHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.SecretToken = "1234"
		rec := requestToConfigAgentHandler(t, cfg)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		tests.AssertApproveResult(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.SecretToken = "1234"
		h, err := configAgentHandler(cfg, beatertest.NilReporter)
		require.NoError(t, err)
		c, rec := beatertest.DefaultContextWithResponseRecorder()
		c.Request.Header.Set(headers.Authorization, "Bearer 1234")
		h(c)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		tests.AssertApproveResult(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("Off", func(t *testing.T) {
		rec := requestToConfigAgentHandler(t, config.DefaultConfig(beatertest.MockBeatVersion()))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		tests.AssertApproveResult(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())

	})

	t.Run("On", func(t *testing.T) {
		rec := requestToConfigAgentHandler(t, configEnabledConfigAgent())

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		tests.AssertApproveResult(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_PanicMiddleware(t *testing.T) {
	h, err := configAgentHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	rec := &beatertest.WriterPanicOnce{}
	c := &request.Context{}
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h(c)
	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	tests.AssertApproveResult(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
}

func TestConfigAgentHandler_MonitoringMiddleware(t *testing.T) {
	h, err := configAgentHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	c, _ := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")

	expected := map[request.ResultID]int{
		request.IDRequestCount:            1,
		request.IDResponseCount:           1,
		request.IDResponseErrorsCount:     1,
		request.IDResponseErrorsForbidden: 1}
	equal, result := beatertest.CompareMonitoringInt(h, c, expected, agent.MonitoringMap)
	assert.True(t, equal, result)

}

func requestToConfigAgentHandler(t *testing.T, cfg *config.Config) *httptest.ResponseRecorder {
	h, err := configAgentHandler(cfg, beatertest.NilReporter)
	require.NoError(t, err)
	c, rec := beatertest.DefaultContextWithResponseRecorder()
	h(c)
	return rec
}

func configEnabledConfigAgent() *config.Config {
	cfg := config.DefaultConfig(beatertest.MockBeatVersion())
	cfg.Kibana = common.MustNewConfigFrom(map[string]interface{}{"enabled": "true"})
	return cfg
}

func approvalPathConfigAgent(f string) string { return "config/agent/test_approved/integration/" + f }
