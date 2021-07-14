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
	"github.com/elastic/apm-server/beater/api/asset/sourcemap"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestSourcemapHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := cfgEnabledRUM()
		cfg.AgentAuth.SecretToken = "1234"
		cfg.AgentAuth.Anonymous.Enabled = false
		rec, err := requestToMuxerWithPattern(cfg, AssetSourcemapPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})
	t.Run("Forbidden", func(t *testing.T) {
		// anonymous access is not allowed for uploading source maps
		cfg := cfgEnabledRUM()
		cfg.AgentAuth.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(cfg, AssetSourcemapPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})
	t.Run("Authorized", func(t *testing.T) {
		// anonymous access is not allowed for uploading source maps
		cfg := cfgEnabledRUM()
		cfg.AgentAuth.SecretToken = "1234"
		cfg.AgentAuth.Anonymous.Enabled = false
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(cfg, AssetSourcemapPath, http.MethodPost, h)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})
}

func TestSourcemapHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("OffRum", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(config.DefaultConfig(), AssetSourcemapPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})

	t.Run("OffSourcemap", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.RumConfig.SourceMapping.Enabled = true
		rec, err := requestToMuxerWithPattern(cfg, AssetSourcemapPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})

	t.Run("DataStreams", func(t *testing.T) {
		cfg := cfgEnabledRUM()
		cfg.DataStreams.Enabled = true
		rec, err := requestToMuxerWithPattern(cfg, AssetSourcemapPath)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})

	t.Run("On", func(t *testing.T) {
		cfg := cfgEnabledRUM()
		cfg.RumConfig.SourceMapping.Enabled = true
		rec, err := requestToMuxerWithPattern(cfg, AssetSourcemapPath)
		require.NoError(t, err)
		require.NotEqual(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathAsset(t.Name()), rec.Body.Bytes())
	})
}

func TestSourcemapHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/assets/v1/sourcemaps", approvalPathAsset(t.Name()))
}

func TestSourcemapHandler_MonitoringMiddleware(t *testing.T) {
	// send GET request resulting in 403 Forbidden error as RUM is disabled by default
	testMonitoringMiddleware(t, "/assets/v1/sourcemaps", sourcemap.MonitoringMap, map[request.ResultID]int{
		request.IDRequestCount:            1,
		request.IDResponseCount:           1,
		request.IDResponseErrorsCount:     1,
		request.IDResponseErrorsForbidden: 1,
	})
}

func approvalPathAsset(f string) string { return "asset/sourcemap/test_approved/integration/" + f }
