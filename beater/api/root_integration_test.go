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

	"github.com/elastic/apm-server/beater/api/root"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/tests"
)

func TestRootHandler_AuthorizationMiddleware(t *testing.T) {
	cfg := config.DefaultConfig(beatertest.MockBeatVersion())
	cfg.SecretToken = "1234"
	h, err := rootHandler(cfg, beatertest.NilReporter)
	require.NoError(t, err)

	t.Run("No auth", func(t *testing.T) {
		c, rec := beatertest.ContextWithResponseRecorder(http.MethodGet, "/")
		h(c)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("Authorized", func(t *testing.T) {
		c, rec := beatertest.ContextWithResponseRecorder(http.MethodGet, "/")
		c.Request.Header.Set(headers.Authorization, "Bearer 1234")
		h(c)

		assert.Equal(t, http.StatusOK, rec.Code)
		tests.AssertApproveResult(t, approvalPathRoot(t.Name()), rec.Body.Bytes())
	})
}

func TestRootHandler_PanicMiddleware(t *testing.T) {
	h, err := rootHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	rec := &beatertest.WriterPanicOnce{}
	c := &request.Context{}
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h(c)

	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	tests.AssertApproveResult(t, approvalPathRoot(t.Name()), rec.Body.Bytes())
}

func TestRootHandler_MonitoringMiddleware(t *testing.T) {
	h, err := rootHandler(config.DefaultConfig(beatertest.MockBeatVersion()), beatertest.NilReporter)
	require.NoError(t, err)
	c, _ := beatertest.ContextWithResponseRecorder(http.MethodGet, "/")

	// send GET request resulting in 403 Forbidden error as RUM is disabled by default
	expected := map[request.ResultID]int{
		request.IDRequestCount:       1,
		request.IDResponseCount:      1,
		request.IDResponseValidCount: 1,
		request.IDResponseValidOK:    1}

	equal, result := beatertest.CompareMonitoringInt(h, c, expected, root.MonitoringMap)
	assert.True(t, equal, result)
}

func approvalPathRoot(f string) string { return "root/test_approved/integration/" + f }
