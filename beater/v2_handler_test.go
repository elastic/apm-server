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
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

func TestInvalidContentType(t *testing.T) {
	req := httptest.NewRequest("POST", V2BackendURL, nil)
	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler("", c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestEmptyRequest(t *testing.T) {
	req := httptest.NewRequest("POST", V2BackendURL, nil)
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler("", c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestRequestDecoderError(t *testing.T) {
	req := httptest.NewRequest("POST", V2BackendURL, bytes.NewBufferString(`asdasd`))
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	expectedErr := errors.New("Faulty decoder")
	faultyDecoder := func(r *http.Request) (map[string]interface{}, error) {
		return nil, expectedErr
	}
	testRouteWithFaultyDecoder := v2Route{
		routeType{
			v2backendHandler,
			func(*Config, decoder.ReqDecoder) decoder.ReqDecoder { return faultyDecoder },
			func(*Config) transform.Config { return transform.Config{} },
		},
	}

	handler := testRouteWithFaultyDecoder.Handler("", c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

func TestRequestIntegration(t *testing.T) {
	for _, test := range []struct {
		name         string
		code         int
		path         string
		reportingErr error
		counter      *monitoring.Int
	}{
		{name: "Success", code: http.StatusAccepted, path: "errors.ndjson", counter: responseAccepted},
		{name: "InvalidEvent", code: http.StatusBadRequest, path: "invalid-event.ndjson", counter: validateCounter},
		{name: "InvalidJSONEvent", code: http.StatusBadRequest, path: "invalid-json-event.ndjson", counter: validateCounter},
		{name: "InvalidJSONMetadata", code: http.StatusBadRequest, path: "invalid-json-metadata.ndjson", counter: validateCounter},
		{name: "InvalidMetadata", code: http.StatusBadRequest, path: "invalid-metadata.ndjson", counter: validateCounter},
		{name: "InvalidMetadata2", code: http.StatusBadRequest, path: "invalid-metadata-2.ndjson", counter: validateCounter},
		{name: "UnrecognizedEvent", code: http.StatusBadRequest, path: "unrecognized-event.ndjson", counter: validateCounter},
		{name: "Closing", code: http.StatusServiceUnavailable, path: "errors.ndjson", reportingErr: publish.ErrChannelClosed, counter: serverShuttingDownCounter},
		{name: "FullQueue", code: http.StatusServiceUnavailable, path: "errors.ndjson", reportingErr: publish.ErrFull, counter: fullQueueCounter},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctSuccess := responseSuccesses.Get()
			ctFailure := responseErrors.Get()
			ct := test.counter.Get()

			w, err := sendReq(defaultConfig("7.0.0"),
				&v2BackendRoute,
				V2BackendURL,
				filepath.Join("../testdata/intake-v2/", test.path),
				test.reportingErr)
			require.NoError(t, err)

			assert.Equal(t, test.code, w.Code, w.Body.String())
			assert.Equal(t, ct+1, test.counter.Get())
			if test.code == http.StatusAccepted {
				assert.Equal(t, 0, w.Body.Len())
				assert.Equal(t, w.HeaderMap.Get("Content-Type"), "")
				assert.Equal(t, ctSuccess+1, responseSuccesses.Get())
				assert.Equal(t, ctFailure, responseErrors.Get())
			} else {
				assert.Equal(t, "application/json", w.HeaderMap.Get("Content-Type"))
				assert.Equal(t, ctSuccess, responseSuccesses.Get())
				assert.Equal(t, ctFailure+1, responseErrors.Get())

				body := w.Body.Bytes()
				tests.AssertApproveResult(t, "approved-stream-result/TestRequestIntegration"+test.name, body)
			}
		})
	}
}

func TestRequestIntegrationRUM(t *testing.T) {
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
			c, err := NewConfig("7.0.0", ucfg)
			require.NoError(t, err)
			w, err := sendReq(c, &v2RumRoute, V2RumURL, test.path, nil)
			require.NoError(t, err)

			require.Equal(t, test.code, w.Code, w.Body.String())
			if test.code != http.StatusAccepted {
				body := w.Body.Bytes()
				tests.AssertApproveResult(t, "approved-stream-result/TestRequestIntegrationRum"+test.name, body)
			}
		})
	}
}

func sendReq(c *Config, route *v2Route, url string, p string, repErr error) (*httptest.ResponseRecorder, error) {
	b, err := loader.LoadDataAsBytes(p)
	if err != nil {
		return nil, err
	}
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")

	report := func(context.Context, publish.PendingReq) error {
		return repErr
	}
	handler := route.Handler(url, c, report)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w, nil
}

func TestV2WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/intake/v2/events", nil)
	req.Header.Add("Content-Type", "application/x-ndjson")
	w := httptest.NewRecorder()
	handler := (&v2BackendRoute).Handler("", defaultConfig("7.0.0"), nil)

	ct := methodNotAllowedCounter.Get()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, ct+1, methodNotAllowedCounter.Get())
}

func TestV2LineExceeded(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/v2/intake", bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	nilReport := func(ctx context.Context, p publish.PendingReq) error { return nil }

	c := defaultConfig("7.0.0")
	assert.False(t, lineLimitExceededInTestData(c.MaxEventSize))
	handler := (&v2BackendRoute).Handler("", c, nilReport)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	assert.Equal(t, 0, w.Body.Len())

	c.MaxEventSize = 20
	assert.True(t, lineLimitExceededInTestData(c.MaxEventSize))
	handler = (&v2BackendRoute).Handler("", c, nilReport)

	req = httptest.NewRequest("POST", "/v2/intake", bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")
	w = httptest.NewRecorder()

	ct := requestTooLargeCounter.Get()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, ct+1, requestTooLargeCounter.Get())
	tests.AssertApproveResult(t, "approved-stream-result/TestV2LineExceeded", w.Body.Bytes())
}
