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

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api/config/agent"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestConfigAgentHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.AgentAuth.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(cfg, AgentConfigPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.AgentAuth.SecretToken = "1234"
		header := map[string]string{headers.Authorization: "Bearer 1234"}
		queryString := map[string]string{"service.name": "service1"}
		rec, err := requestToMuxerWithHeaderAndQueryString(cfg, AgentConfigPath, http.MethodGet, header, queryString)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("On", func(t *testing.T) {
		queryString := map[string]string{"service.name": "service1"}
		rec, err := requestToMuxerWithHeaderAndQueryString(configEnabledConfigAgent(), AgentConfigPath, http.MethodGet, nil, queryString)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_DirectConfiguration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AgentConfigs = []config.AgentConfig{
		{
			Service: config.Service{Name: "service1", Environment: ""},
			Config:  map[string]string{"key1": "val1"},
			Etag:    "abc123",
		},
	}

	mux, err := muxBuilder{Managed: true}.build(cfg)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, AgentConfigPath, nil)
	r = requestWithQueryString(r, map[string]string{"service.name": "service1"})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), w.Body.Bytes())

}

func TestConfigAgentHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/config/v1/agents", approvalPathConfigAgent(t.Name()))
}

func TestConfigAgentHandler_MonitoringMiddleware(t *testing.T) {
	testMonitoringMiddleware(t, "/config/v1/agents", agent.MonitoringMap, map[request.ResultID]int{
		request.IDRequestCount:               1,
		request.IDResponseCount:              1,
		request.IDResponseErrorsCount:        1,
		request.IDResponseErrorsInvalidQuery: 1,
	})
}

func configEnabledConfigAgent() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Kibana.Enabled = true
	cfg.Kibana.Host = "localhost:foo"
	return cfg
}

func approvalPathConfigAgent(f string) string { return "config/agent/test_approved/integration/" + f }
