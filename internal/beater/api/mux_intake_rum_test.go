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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/middleware"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestOPTIONS(t *testing.T) {
	ratelimitStore, _ := ratelimit.NewStore(1, 1, 1)
	requestTaken := make(chan struct{}, 1)
	done := make(chan struct{}, 1)
	handled := make(chan struct{}, 2) // 2 expected request

	cfg := cfgEnabledRUM()
	cfg.RumConfig.AllowOrigins = []string{"*"}
	authenticator, _ := auth.NewAuthenticator(cfg.AgentAuth, logptest.NewTestingLogger(t, ""))

	lastMiddleware := func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			h(c)
			handled <- struct{}{}
		}, nil
	}

	h, _ := middleware.Wrap(
		func(c *request.Context) {
			requestTaken <- struct{}{}
			<-done
		},
		append([]middleware.Middleware{lastMiddleware}, rumMiddleware(cfg, authenticator, ratelimitStore, "", noop.NewMeterProvider())...)...)

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
	done <- struct{}{} // unblock first request
	// wait for both request middlewares to complete
	<-handled
	<-handled
}

func TestRUMHandler_NoAuthorizationRequired(t *testing.T) {
	cfg := cfgEnabledRUM()
	cfg.AgentAuth.SecretToken = "1234"
	rec, err := requestToMuxerWithPattern(t, cfg, IntakeRUMPath)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestRUMHandler_KillSwitchMiddleware(t *testing.T) {
	t.Run("OffRum", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(t, config.DefaultConfig(), IntakeRUMPath)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		expected, err := os.ReadFile(approvalPathIntakeRUM(t.Name()) + ".approved.json")
		require.NoError(t, err)
		assert.JSONEq(t, string(expected), rec.Body.String())
	})

	t.Run("On", func(t *testing.T) {
		rec, err := requestToMuxerWithPattern(t, cfgEnabledRUM(), IntakeRUMPath)
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, rec.Code)
	})
}

func TestRUMHandler_CORSMiddleware(t *testing.T) {
	cfg := cfgEnabledRUM()
	cfg.RumConfig.AllowOrigins = []string{"foo"}
	h, _ := newTestMux(t, cfg)

	for _, path := range []string{"/intake/v2/rum/events", "/intake/v3/rum/events"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(headers.Origin, "bar")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestIntakeRUMHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/intake/v2/rum/events")
	testPanicMiddleware(t, "/intake/v3/rum/events")
}

func TestRumHandler_MonitoringMiddleware(t *testing.T) {
	// send GET request resulting in 403 Forbidden error
	for _, path := range []string{"/intake/v2/rum/events", "/intake/v3/rum/events"} {
		testMonitoringMiddleware(t, path, map[string]any{
			"http.server." + string(request.IDRequestCount):            1,
			"http.server." + string(request.IDResponseCount):           1,
			"http.server." + string(request.IDResponseErrorsCount):     1,
			"http.server." + string(request.IDResponseErrorsForbidden): 1,
		})
	}
}

func cfgEnabledRUM() *config.Config {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentDecoders = 10
	cfg.RumConfig.Enabled = true
	cfg.AgentAuth.Anonymous.Enabled = true
	return cfg
}

func approvalPathIntakeRUM(f string) string {
	return "intake/test_approved/integration/rum/" + f
}
