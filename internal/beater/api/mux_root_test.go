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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/api/root"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/version"
)

func TestRootHandler_AuthorizationMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AgentAuth.SecretToken = "1234"

	t.Run("No auth", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(cfg, RootPath)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("Authorized", func(t *testing.T) {
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(cfg, RootPath, http.MethodGet, h)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "build_date")

		var result struct {
			Version string
		}
		err = json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, version.Version, result.Version)
	})
}

func TestRootHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/")
}

func TestRootHandler_MonitoringMiddleware(t *testing.T) {
	testMonitoringMiddleware(t, "/", root.MonitoringMap, map[request.ResultID]int{
		request.IDRequestCount:       1,
		request.IDResponseCount:      1,
		request.IDResponseValidCount: 1,
		request.IDResponseValidOK:    1,
	})
}
