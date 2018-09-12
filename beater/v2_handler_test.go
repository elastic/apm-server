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
	"testing"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
)

func TestInvalidContentType(t *testing.T) {
	req := httptest.NewRequest("POST", V2BackendURL, nil)
	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestEmptyRequest(t *testing.T) {
	req := httptest.NewRequest("POST", V2BackendURL, nil)
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

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

	handler := testRouteWithFaultyDecoder.Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

func TestRequestIntegration(t *testing.T) {
	for _, test := range []struct {
		name         string
		code         int
		path         string
		reportingErr error
	}{
		{name: "Success", code: http.StatusAccepted, path: "../testdata/intake-v2/errors.ndjson"},
		{name: "InvalidEvent", code: http.StatusBadRequest, path: "../testdata/intake-v2/invalid-event.ndjson"},
		{name: "InvalidJSONEvent", code: http.StatusBadRequest, path: "../testdata/intake-v2/invalid-json-event.ndjson"},
		{name: "InvalidJSONMetadata", code: http.StatusBadRequest, path: "../testdata/intake-v2/invalid-json-metadata.ndjson"},
		{name: "InvalidMetadata", code: http.StatusBadRequest, path: "../testdata/intake-v2/invalid-metadata.ndjson"},
		{name: "InvalidMetadata2", code: http.StatusBadRequest, path: "../testdata/intake-v2/invalid-metadata-2.ndjson"},
		{name: "UnrecognizedEvent", code: http.StatusBadRequest, path: "../testdata/intake-v2/unrecognized-event.ndjson"},
		{name: "Closing", code: http.StatusServiceUnavailable, path: "../testdata/intake-v2/errors.ndjson", reportingErr: publish.ErrChannelClosed},
		{name: "FullQueue", code: http.StatusTooManyRequests, path: "../testdata/intake-v2/errors.ndjson", reportingErr: publish.ErrFull},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(test.path)
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			req := httptest.NewRequest("POST", V2BackendURL, bodyReader)
			req.Header.Add("Content-Type", "application/x-ndjson")

			w := httptest.NewRecorder()

			c := defaultConfig("7.0.0")
			report := func(context.Context, publish.PendingReq) error {
				return test.reportingErr
			}
			handler := (&v2BackendRoute).Handler(c, report)

			handler.ServeHTTP(w, req)

			assert.Equal(t, test.code, w.Code, w.Body.String())
			if test.code == http.StatusAccepted {
				assert.Equal(t, 0, w.Body.Len())
				assert.Equal(t, w.HeaderMap.Get("Content-Type"), "")
			} else {
				assert.Equal(t, w.HeaderMap.Get("Content-Type"), "application/json")

				body := w.Body.Bytes()
				tests.AssertApproveResult(t, "approved-stream-result/TestRequestIntegration"+test.name, body)
			}
		})
	}
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

	report := func(ctx context.Context, p publish.PendingReq) error {
		return nil
	}

	c := defaultConfig("7.0.0")
	assert.False(t, lineLimitExceededInTestData(c.MaxEventSize))
	handler := (&v2BackendRoute).Handler(c, report)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	assert.Equal(t, 0, w.Body.Len())

	c.MaxEventSize = 20
	assert.True(t, lineLimitExceededInTestData(c.MaxEventSize))
	handler = (&v2BackendRoute).Handler(c, report)

	req = httptest.NewRequest("POST", "/v2/intake", bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/x-ndjson")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	tests.AssertApproveResult(t, "approved-stream-result/TestV2LineExceeded", w.Body.Bytes())
}
