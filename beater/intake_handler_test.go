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

package beater

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
)

func TestInvalidContentType(t *testing.T) {
	h, err := backendHandler(defaultConfig("7.0.0"), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", intakePath, nil)
	ctx := &request.Context{}
	ctx.Reset(w, req)
	h(ctx)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid content type: ''")
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get(headers.ContentType))
}

func TestEmptyRequest(t *testing.T) {
	req := httptest.NewRequest("POST", intakePath, nil)
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	handler, err := backendHandler(defaultConfig("7.0.0"), nil)
	require.NoError(t, err)
	ctx := &request.Context{}
	ctx.Reset(w, req)
	handler(ctx)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestRequestIntegration(t *testing.T) {
	requestCounter := intakeResultIDToMonitoringInt(request.IDRequestCount)
	responseSuccesses := intakeResultIDToMonitoringInt(request.IDResponseValidCount)
	responseErrors := intakeResultIDToMonitoringInt(request.IDResponseErrorsCount)
	responseAccepted := intakeResultIDToMonitoringInt(request.IDResponseValidAccepted)
	validateCounter := intakeResultIDToMonitoringInt(request.IDResponseErrorsValidate)
	serverShuttingDownCounter := intakeResultIDToMonitoringInt(request.IDResponseErrorsShuttingDown)
	fullQueueCounter := intakeResultIDToMonitoringInt(request.IDResponseErrorsFullQueue)

	endpoints := []struct {
		name, url string
		fn        func(*Config, publish.Reporter) (request.Handler, error)
	}{
		{name: "Backend", url: intakePath, fn: backendHandler},
		{name: "Rum", url: intakeRumPath, fn: rumHandler},
	}
	for name, test := range map[string]struct {
		code         int
		path         string
		reportingErr error
		counter      *monitoring.Int
	}{
		"Success":             {code: http.StatusAccepted, path: "errors.ndjson", counter: responseAccepted},
		"InvalidEvent":        {code: http.StatusBadRequest, path: "invalid-event.ndjson", counter: validateCounter},
		"InvalidJSONEvent":    {code: http.StatusBadRequest, path: "invalid-json-event.ndjson", counter: validateCounter},
		"InvalidJSONMetadata": {code: http.StatusBadRequest, path: "invalid-json-metadata.ndjson", counter: validateCounter},
		"InvalidMetadata":     {code: http.StatusBadRequest, path: "invalid-metadata.ndjson", counter: validateCounter},
		"InvalidMetadata2":    {code: http.StatusBadRequest, path: "invalid-metadata-2.ndjson", counter: validateCounter},
		"UnrecognizedEvent":   {code: http.StatusBadRequest, path: "unrecognized-event.ndjson", counter: validateCounter},
		"Closing":             {code: http.StatusServiceUnavailable, path: "errors.ndjson", reportingErr: publish.ErrChannelClosed, counter: serverShuttingDownCounter},
		"FullQueue":           {code: http.StatusServiceUnavailable, path: "errors.ndjson", reportingErr: publish.ErrFull, counter: fullQueueCounter},
	} {

		for _, endpoint := range endpoints {
			testname := name + endpoint.name
			cfg := defaultConfig("7.0.0")
			rum := true
			cfg.RumConfig.Enabled = &rum

			if endpoint.name == "Rum" {
				rum := true
				cfg.RumConfig.Enabled = &rum
			}

			// reset counters
			ctSuccess := responseSuccesses.Get()
			ctFailure := responseErrors.Get()
			ct := test.counter.Get()
			reqCt := requestCounter.Get()

			// Send request
			w, err := sendReq(cfg,
				endpoint.fn,
				endpoint.url,
				filepath.Join("../testdata/intake-v2/", test.path),
				test.reportingErr)
			require.NoError(t, err)

			t.Run(testname+"Status", func(t *testing.T) {
				assert.Equal(t, test.code, w.Code, w.Body.String())
			})

			t.Run(testname+"Header", func(t *testing.T) {
				assert.Equal(t, "application/json", w.Header().Get(headers.ContentType))
				contentLength := w.Header().Get(headers.ContentLength)
				require.NotEmpty(t, contentLength)
				cl, err := strconv.Atoi(contentLength)
				require.NoError(t, err)
				assert.True(t, cl > 0)
			})

			t.Run(testname+"Body", func(t *testing.T) {
				if test.code == http.StatusAccepted {
					assert.NotNil(t, w.Body.Len())
				}
				body := w.Body.Bytes()
				//TODO: ensure that approval tests for backend and rum don't overwrite each other
				tests.AssertApproveResult(t, "test_approved_stream_result/TestRequestIntegration"+name, body)
			})

			t.Run(testname+"MonitoringCounter", func(t *testing.T) {
				assert.Equal(t, ct+1, test.counter.Get())
				assert.Equal(t, reqCt+1, requestCounter.Get())
				if test.code == http.StatusAccepted {
					assert.Equal(t, ctSuccess+1, responseSuccesses.Get())
					assert.Equal(t, ctFailure, responseErrors.Get())
				} else {
					assert.Equal(t, ctSuccess, responseSuccesses.Get())
					assert.Equal(t, ctFailure+1, responseErrors.Get())
				}
			})
		}
	}
}

func TestRequestIntegrationRUM(t *testing.T) {
	type m map[string]interface{}

	for _, test := range []struct {
		name string
		code int
		path string
	}{
		{name: "Success", code: http.StatusAccepted, path: "../testdata/intake-v2/errors.ndjson"},
		{name: "RateLimit", code: http.StatusTooManyRequests, path: "../testdata/intake-v2/heavy.ndjson"},
	} {
		t.Run(test.name, func(t *testing.T) {

			ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "event_rate": m{"limit": 9}}})
			require.NoError(t, err)
			c, err := newConfig("7.0.0", ucfg)
			require.NoError(t, err)
			w, err := sendReq(c, rumHandler, intakeRumPath, test.path, nil)
			require.NoError(t, err)

			require.Equal(t, test.code, w.Code, w.Body.String())
			if test.code != http.StatusAccepted {
				body := w.Body.Bytes()
				tests.AssertApproveResult(t, "test_approved_stream_result/TestRequestIntegrationRum"+test.name, body)
			}
		})
	}
}

func sendReq(c *Config, handlerFn func(*Config, publish.Reporter) (request.Handler, error), url string, p string, repErr error) (*httptest.ResponseRecorder, error) {
	b, err := loader.LoadDataAsBytes(p)
	if err != nil {
		return nil, err
	}
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")
	req.Header.Add("Accept", "application/json")
	q := req.URL.Query()
	q.Add("verbose", "")
	req.URL.RawQuery = q.Encode()

	report := func(context.Context, publish.PendingReq) error {
		return repErr
	}
	handler, err := handlerFn(c, report)
	if err != nil {
		return nil, err
	}

	w := httptest.NewRecorder()
	newContextPool().handler(handler).ServeHTTP(w, req)
	return w, nil
}

func TestWrongMethod(t *testing.T) {
	registry := monitoring.Default.NewRegistry(t.Name()+"test", monitoring.PublishExpvar)
	methodNotAllowedCounter := monitoring.NewInt(registry, string(request.IDResponseErrorsMethodNotAllowed))
	monitoringFn := func(id request.ResultID) *monitoring.Int {
		if id == request.IDResponseErrorsMethodNotAllowed {
			return methodNotAllowedCounter
		}
		return nil
	}

	req := httptest.NewRequest("GET", "/intake/v2/events", nil)
	req.Header.Add("Content-Type", "application/x-ndjson")
	w := httptest.NewRecorder()
	handler, err := backendHandler(defaultConfig("7.0.0"), nil)
	require.NoError(t, err)

	ct := methodNotAllowedCounter.Get()
	ctx := &request.Context{}
	ctx.Reset(w, req)
	withMiddleware(handler, monitoringHandler(monitoringFn))(ctx)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, ct+1, methodNotAllowedCounter.Get())
}

func TestLineExceeded(t *testing.T) {
	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)

	lineLimitExceededInTestData := func(lineLimit int) bool {
		var limitExceeded bool
		for _, l := range bytes.Split(b, []byte("\n")) {
			if len(l) > lineLimit {
				limitExceeded = true
				break
			}
		}
		return limitExceeded
	}

	req := httptest.NewRequest("POST", "/intake/v2/events", bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")
	w := httptest.NewRecorder()

	nilReport := func(ctx context.Context, p publish.PendingReq) error { return nil }
	c := defaultConfig("7.0.0")

	t.Run("OK", func(t *testing.T) {
		assert.False(t, lineLimitExceededInTestData(c.MaxEventSize))
		handler, err := backendHandler(defaultConfig("7.0.0"), nilReport)
		require.NoError(t, err)
		ctx := &request.Context{}
		ctx.Reset(w, req)
		handler(ctx)

		assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
		assert.Equal(t, 0, w.Body.Len())
	})

	t.Run("Exceeded", func(t *testing.T) {
		registry := monitoring.Default.NewRegistry(t.Name()+"test", monitoring.PublishExpvar)
		requestTooLargeCounter := monitoring.NewInt(registry, string(request.IDResponseErrorsRequestTooLarge))
		monitoringFn := func(id request.ResultID) *monitoring.Int {
			if id == request.IDResponseErrorsRequestTooLarge {
				return requestTooLargeCounter
			}
			return nil
		}

		c := defaultConfig("7.0.0")
		c.MaxEventSize = 20
		assert.True(t, lineLimitExceededInTestData(c.MaxEventSize))
		handler, err := backendHandler(c, nilReport)
		require.NoError(t, err)

		req = httptest.NewRequest("POST", "/intake/v2/events", bytes.NewBuffer(b))
		req.Header.Add("Content-Type", "application/x-ndjson")
		req.Header.Add("Accept", "*/*")
		w = httptest.NewRecorder()

		ct := requestTooLargeCounter.Get()
		ctx := &request.Context{}
		ctx.Reset(w, req)
		withMiddleware(handler, monitoringHandler(monitoringFn))(ctx)
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, ct+1, requestTooLargeCounter.Get())
		tests.AssertApproveResult(t, "test_approved_stream_result/TestLineExceeded", w.Body.Bytes())
	})
}

func TestMonitoringHandler_IntakeBackend(t *testing.T) {
	requestCounter := intakeResultIDToMonitoringInt(request.IDRequestCount)
	responseCounter := intakeResultIDToMonitoringInt(request.IDResponseCount)
	responseErrors := intakeResultIDToMonitoringInt(request.IDResponseErrorsCount)
	responseSuccesses := intakeResultIDToMonitoringInt(request.IDResponseValidCount)

	t.Run("Error", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/events/")
		withMiddleware(mockHandler403, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseErrors: 1,
			intakeResultIDToMonitoringInt(request.IDResponseErrorsForbidden): 1})
	})
	t.Run("Accepted", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/events/")
		withMiddleware(mockHandler202, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1,
			intakeResultIDToMonitoringInt(request.IDResponseValidAccepted): 1})
	})
	t.Run("Idle", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/events")

		withMiddleware(mockHandlerIdle, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1})
	})
}

func TestMonitoringHandler_IntakeRUM(t *testing.T) {
	requestCounter := intakeResultIDToMonitoringInt(request.IDRequestCount)
	responseCounter := intakeResultIDToMonitoringInt(request.IDResponseCount)
	responseErrors := intakeResultIDToMonitoringInt(request.IDResponseErrorsCount)
	responseSuccesses := intakeResultIDToMonitoringInt(request.IDResponseValidCount)

	t.Run("Error", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/rum/events/")
		withMiddleware(mockHandler403, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseErrors: 1,
			intakeResultIDToMonitoringInt(request.IDResponseErrorsForbidden): 1})
	})
	t.Run("Accepted", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/rum/events/")
		withMiddleware(mockHandler202, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1,
			intakeResultIDToMonitoringInt(request.IDResponseValidAccepted): 1})
	})
	t.Run("Idle", func(t *testing.T) {
		testResetCounter()
		c := setupContext("/intake/v2/rum/events")

		withMiddleware(mockHandlerIdle, monitoringHandler(intakeResultIDToMonitoringInt))(c)
		testCounter(t, map[*monitoring.Int]int64{requestCounter: 1,
			responseCounter: 1, responseSuccesses: 1})
	})
}
