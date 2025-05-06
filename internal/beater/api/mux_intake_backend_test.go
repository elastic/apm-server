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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
)

func TestIntakeBackendHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.AgentAuth.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(t, cfg, IntakePath)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		expected, err := os.ReadFile(approvalPathIntakeBackend(t.Name()) + ".approved.json")
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), rec.Body.String())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.AgentAuth.SecretToken = "1234"
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(t, cfg, IntakePath, http.MethodGet, h)
		require.NoError(t, err)

		require.NotEqual(t, http.StatusUnauthorized, rec.Code)

		expected, err := os.ReadFile(approvalPathIntakeBackend(t.Name()) + ".approved.json")
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), rec.Body.String())
	})
}

func TestIntakeBackendHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/intake/v2/events")
}

func TestIntakeBackendHandler_MonitoringMiddleware(t *testing.T) {
	// send GET request resulting in 405 MethodNotAllowed error
	testMonitoringMiddleware(t, "/intake/v2/events", map[string]any{
		"http.server." + string(request.IDRequestCount):                   1,
		"http.server." + string(request.IDResponseCount):                  1,
		"http.server." + string(request.IDResponseErrorsCount):            1,
		"http.server." + string(request.IDResponseErrorsMethodNotAllowed): 1,
	})
}

func approvalPathIntakeBackend(f string) string {
	return "intake/test_approved/integration/backend/" + f
}
