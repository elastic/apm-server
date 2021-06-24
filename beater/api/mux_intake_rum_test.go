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
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/middleware"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/beater/request"
)

func TestOPTIONS(t *testing.T) {
	ratelimitStore, _ := ratelimit.NewStore(1, 1, 1)
	requestTaken := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	cfg := cfgEnabledRUM()
	cfg.RumConfig.AllowOrigins = []string{"*"}
	h, _ := middleware.Wrap(
		func(c *request.Context) {
			requestTaken <- struct{}{}
			<-done
		},
		rumMiddleware(cfg, nil, ratelimitStore, intake.MonitoringMap)...)

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
	cfg.AgentAuth.SecretToken = "1234"
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
	h := newTestMux(t, cfg)

	for _, path := range []string{"/intake/v2/rum/events", "/intake/v3/rum/events"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(headers.Origin, "bar")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestIntakeRUMHandler_PanicMiddleware(t *testing.T) {
	testPanicMiddleware(t, "/intake/v2/rum/events", approvalPathIntakeRUM(t.Name()))
	testPanicMiddleware(t, "/intake/v3/rum/events", approvalPathIntakeRUM(t.Name()))
}

func TestRumHandler_MonitoringMiddleware(t *testing.T) {
	// send GET request resulting in 403 Forbidden error
	for _, path := range []string{"/intake/v2/rum/events", "/intake/v3/rum/events"} {
		testMonitoringMiddleware(t, path, intake.MonitoringMap, map[request.ResultID]int{
			request.IDRequestCount:            1,
			request.IDResponseCount:           1,
			request.IDResponseErrorsCount:     1,
			request.IDResponseErrorsForbidden: 1,
		})
	}
}

func TestRUMHandler_AllowedServiceNames(t *testing.T) {
	payload, err := ioutil.ReadFile("../../testdata/intake-v2/transactions_spans_rum.ndjson")
	require.NoError(t, err)

	for _, test := range []struct {
		AllowServiceNames []string
		Allowed           bool
	}{{
		AllowServiceNames: nil,
		Allowed:           true, // none specified = all allowed
	}, {
		AllowServiceNames: []string{"apm-agent-js"}, // matches what's in test data
		Allowed:           true,
	}, {
		AllowServiceNames: []string{"reject_everything"},
		Allowed:           false,
	}} {
		cfg := cfgEnabledRUM()
		cfg.RumConfig.AllowServiceNames = test.AllowServiceNames
		h := newTestMux(t, cfg)

		req := httptest.NewRequest(http.MethodPost, "/intake/v2/rum/events", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/x-ndjson")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if test.Allowed {
			assert.Equal(t, http.StatusAccepted, w.Code)
		} else {
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	}
}

func cfgEnabledRUM() *config.Config {
	cfg := config.DefaultConfig()
	cfg.RumConfig.Enabled = true
	return cfg
}

func approvalPathIntakeRUM(f string) string {
	return "intake/test_approved/integration/rum/" + f
}
