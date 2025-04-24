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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/version"
)

func TestRootHandler_AuthorizationMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AgentAuth.SecretToken = "1234"

	t.Run("No auth", func(t *testing.T) {
		rec, err := requestToMuxerWithHeader(t, cfg, RootPath, http.MethodGet, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("Authorized", func(t *testing.T) {
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(t, cfg, RootPath, http.MethodGet, h)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "build_date")

		var result struct {
			Version string
		}
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, version.VersionWithQualifier(), result.Version)
	})
}

func TestRootHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/")
}

func TestRootHandler_MonitoringMiddleware(t *testing.T) {
	validMethodMetrics := map[string]any{
		"http.server." + string(request.IDRequestCount):           1,
		"http.server." + string(request.IDResponseCount):          1,
		"http.server." + string(request.IDResponseValidCount):     1,
		"http.server." + string(request.IDResponseValidOK):        1,
		"apm-server.root." + string(request.IDRequestCount):       1,
		"apm-server.root." + string(request.IDResponseCount):      1,
		"apm-server.root." + string(request.IDResponseValidCount): 1,
		"apm-server.root." + string(request.IDResponseValidOK):    1,
	}
	invalidMethodMetrics := map[string]any{
		"http.server." + string(request.IDRequestCount):                       1,
		"http.server." + string(request.IDResponseCount):                      1,
		"http.server." + string(request.IDResponseErrorsCount):                1,
		"http.server." + string(request.IDResponseErrorsMethodNotAllowed):     1,
		"apm-server.root." + string(request.IDRequestCount):                   1,
		"apm-server.root." + string(request.IDResponseCount):                  1,
		"apm-server.root." + string(request.IDResponseErrorsCount):            1,
		"apm-server.root." + string(request.IDResponseErrorsMethodNotAllowed): 1,
	}
	for _, tc := range []struct {
		method      string
		wantMetrics map[string]any
	}{
		{
			method:      http.MethodGet,
			wantMetrics: validMethodMetrics,
		},
		{
			method:      http.MethodHead,
			wantMetrics: validMethodMetrics,
		},
		{
			method:      http.MethodPut,
			wantMetrics: invalidMethodMetrics,
		},
		{
			method:      http.MethodPost,
			wantMetrics: invalidMethodMetrics,
		},
	} {
		t.Run(tc.method, func(t *testing.T) {
			h, reader := newTestMux(t, config.DefaultConfig())
			req := httptest.NewRequest(tc.method, "/", nil)
			h.ServeHTTP(httptest.NewRecorder(), req)

			monitoringtest.ExpectContainOtelMetrics(t, reader, tc.wantMetrics)
		})
	}
}
