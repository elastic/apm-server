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

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/request"
)

func TestOPTIONS(t *testing.T) {
	requestTaken := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	cfg := cfgEnabledRUM()
	cfg.RumConfig.AllowOrigins = []string{"*"}
	h, _ := middleware.Wrap(
		func(c *request.Context) {
			requestTaken <- struct{}{}
			<-done
		},
		rumMiddleware(cfg, nil, intake.MonitoringMap)...)

	// use this to block the single allowed concurrent requests
	go func() {
		c := request.NewContext()
		c.Reset(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
		h(c)
	}()

	<-requestTaken

	// send a new request which should be allowed through
	c := request.NewContext()
	w := httptest.NewRecorder()
	c.Reset(w, httptest.NewRequest(http.MethodOptions, "/", nil))
	h(c)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	done <- struct{}{}
}

func TestRUMHandler_NoAuthorizationRequired(t *testing.T) {
	cfg := cfgEnabledRUM()
	cfg.SecretToken = "1234"
	rec, err := requestToMuxerWithPattern(cfg, IntakeRUMPath)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
	approvaltest.ApproveJSON(t, approvalPathIntakeRUM(t.Name()), rec.Body.Bytes())
}

func TestRUMHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("OffRum", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(config.DefaultConfig(), IntakeRUMPath)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathIntakeRUM(t.Name()), rec.Body.Bytes())
	})

	t.Run("On", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(cfgEnabledRUM(), IntakeRUMPath)
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, rec.Code)
		approvaltest.ApproveJSON(t, approvalPathIntakeRUM(t.Name()), rec.Body.Bytes())
	})
}

func TestRUMHandler_CORSMiddleware(t *testing.T) {
	cfg := cfgEnabledRUM()
	cfg.RumConfig.AllowOrigins = []string{"foo"}
	h, err := rumIntakeHandler(cfg, nil, beatertest.NilReporter)
	require.NoError(t, err)
	c, w := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")
	c.Request.Header.Set(headers.Origin, "bar")
	h(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestIntakeRUMHandler_PanicMiddleware(t *testing.T) {
	h, err := rumIntakeHandler(config.DefaultConfig(), nil, beatertest.NilReporter)
	require.NoError(t, err)
	rec := &beatertest.WriterPanicOnce{}
	c := request.NewContext()
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h(c)
	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	approvaltest.ApproveJSON(t, approvalPathIntakeRUM(t.Name()), rec.Body.Bytes())
}

func TestRumHandler_MonitoringMiddleware(t *testing.T) {
	h, err := rumIntakeHandler(config.DefaultConfig(), nil, beatertest.NilReporter)
	require.NoError(t, err)
	c, _ := beatertest.ContextWithResponseRecorder(http.MethodPost, "/")
	// send GET request resulting in 403 Forbidden error
	expected := map[request.ResultID]int{
		request.IDRequestCount:            1,
		request.IDResponseCount:           1,
		request.IDResponseErrorsCount:     1,
		request.IDResponseErrorsForbidden: 1}

	equal, result := beatertest.CompareMonitoringInt(h, c, expected, intake.MonitoringMap)
	assert.True(t, equal, result)
}

func cfgEnabledRUM() *config.Config {
	cfg := config.DefaultConfig()
	t := true
	cfg.RumConfig.Enabled = &t
	return cfg
}

func approvalPathIntakeRUM(f string) string {
	return "intake/test_approved/integration/rum/" + f
}
