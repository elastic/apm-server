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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
)

func TestConfigAgentHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.AgentAuth.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(t, cfg, AgentConfigPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.JSONEq(t,
			`{"error":"authentication failed: missing or improperly formatted Authorization header: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'"}`,
			rec.Body.String(),
		)
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := configEnabledConfigAgent()
		cfg.AgentAuth.SecretToken = "1234"

		header := map[string]string{headers.Authorization: "Bearer 1234"}
		queryString := map[string]string{"service.name": "service1"}
		rec, err := requestToMuxerWithHeaderAndQueryString(t, cfg, AgentConfigPath, http.MethodGet, header, queryString)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusUnauthorized, rec.Code)
		assert.JSONEq(t, "{}", rec.Body.String())
	})
}

func TestConfigAgentHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/config/v1/agents")
}

func TestConfigAgentHandler_MonitoringMiddleware(t *testing.T) {
	testMonitoringMiddleware(t, "/config/v1/agents", map[string]any{
		"http.server." + string(request.IDRequestCount):               1,
		"http.server." + string(request.IDResponseCount):              1,
		"http.server." + string(request.IDResponseErrorsCount):        1,
		"http.server." + string(request.IDResponseErrorsInvalidQuery): 1,
	})
}

func configEnabledConfigAgent() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Kibana.Enabled = true
	cfg.Kibana.Host = "localhost:foo"
	return cfg
}
