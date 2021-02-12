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
		cfg.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(cfg, AgentConfigPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.SecretToken = "1234"
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(cfg, AgentConfigPath, http.MethodGet, h)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("Off", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(config.DefaultConfig(), AgentConfigPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())

	})

	t.Run("On", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(configEnabledConfigAgent(), AgentConfigPath)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathConfigAgent(t.Name()), rec.Body.Bytes())
	})
}

func TestConfigAgentHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/config/v1/agents", approvalPathConfigAgent(t.Name()))
}

func TestConfigAgentHandler_MonitoringMiddleware(t *testing.T) {
	testMonitoringMiddleware(t, "/config/v1/agents", agent.MonitoringMap, map[request.ResultID]int{
		request.IDRequestCount:            1,
		request.IDResponseCount:           1,
		request.IDResponseErrorsCount:     1,
		request.IDResponseErrorsForbidden: 1,
	})
}

func configEnabledConfigAgent() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Kibana.Enabled = true
	cfg.Kibana.Host = "localhost:foo"
	return cfg
}

func approvalPathConfigAgent(f string) string { return "config/agent/test_approved/integration/" + f }
