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

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestIntakeBackendHandler_AuthorizationMiddleware(t *testing.T) {
	t.Run("Unauthorized", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.SecretToken = "1234"
		rec, err := requestToMuxerWithPattern(cfg, IntakePath)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathIntakeBackend(t.Name()), rec.Body.Bytes())
	})

	t.Run("Authorized", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.SecretToken = "1234"
		h := map[string]string{headers.Authorization: "Bearer 1234"}
		rec, err := requestToMuxerWithHeader(cfg, IntakePath, http.MethodGet, h)
		require.NoError(t, err)

		require.NotEqual(t, http.StatusUnauthorized, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathIntakeBackend(t.Name()), rec.Body.Bytes())
	})
}

func TestIntakeBackendHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/intake/v2/events", approvalPathIntakeBackend(t.Name()))
}

func TestIntakeBackendHandler_MonitoringMiddleware(t *testing.T) {
	// send GET request resulting in 405 MethodNotAllowed error
	testMonitoringMiddleware(t, "/intake/v2/events", intake.MonitoringMap, map[request.ResultID]int{
		request.IDRequestCount:                   1,
		request.IDResponseCount:                  1,
		request.IDResponseErrorsCount:            1,
		request.IDResponseErrorsMethodNotAllowed: 1,
	})
}

func approvalPathIntakeBackend(f string) string {
	return "intake/test_approved/integration/backend/" + f
}
