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
	"encoding/json"
	"fmt"
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

func approveResultBody(t *testing.T, name string, body *bytes.Buffer) {
	var resultmap map[string]interface{}
	err := json.Unmarshal(body.Bytes(), &resultmap)
	assert.NoError(t, err)

	resultName := fmt.Sprintf("approved-stream-result/%s", name)
	verifyErr := tests.ApproveJson(resultmap, resultName, nil)
	if verifyErr != nil {
		assert.Fail(t, fmt.Sprintf("Test %s failed with error: %s", name, verifyErr.Error()))
	}
}

func TestInvalidContentType(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", nil)
	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestEmptyRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", nil)
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestRequestDecoderError(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", bytes.NewBufferString(`asdasd`))
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
		name string
		code int
		path string
	}{
		{"Success", http.StatusAccepted, "../testdata/intake-v2/errors.ndjson"},
		{"InvalidEvent", http.StatusBadRequest, "../testdata/intake-v2/invalid-event.ndjson"},
		{"InvalidJSONEvent", http.StatusBadRequest, "../testdata/intake-v2/invalid-json-event.ndjson"},
		{"InvalidJSONMetadata", http.StatusBadRequest, "../testdata/intake-v2/invalid-json-metadata.ndjson"},
		{"InvalidMetadata", http.StatusBadRequest, "../testdata/intake-v2/invalid-metadata.ndjson"},
		{"InvalidMetadata2", http.StatusBadRequest, "../testdata/intake-v2/invalid-metadata-2.ndjson"},
		{"UnrecognizedEvent", http.StatusBadRequest, "../testdata/intake-v2/unrecognized-event.ndjson"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes(test.path)
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			req := httptest.NewRequest("POST", "/v2/intake", bodyReader)
			req.Header.Add("Content-Type", "application/x-ndjson")

			w := httptest.NewRecorder()

			c := defaultConfig("7.0.0")
			report := func(context.Context, publish.PendingReq) error {
				return nil
			}
			handler := (&v2BackendRoute).Handler(c, report)

			handler.ServeHTTP(w, req)

			assert.Equal(t, test.code, w.Code, w.Body.String())

			approveResultBody(t, "TestRequestIntegration"+test.name, w.Body)
		})
	}
}

func TestReportingProblemsIntegration(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
		err  error
	}{
		{"Closing", http.StatusServiceUnavailable, publish.ErrChannelClosed},
		{"FullQueue", http.StatusTooManyRequests, publish.ErrFull},
	} {
		t.Run(test.name, func(t *testing.T) {
			b, err := loader.LoadDataAsBytes("../testdata/intake-v2/errors.ndjson")
			require.NoError(t, err)
			bodyReader := bytes.NewBuffer(b)

			req := httptest.NewRequest("POST", "/v2/intake", bodyReader)
			req.Header.Add("Content-Type", "application/x-ndjson")

			w := httptest.NewRecorder()

			c := defaultConfig("7.0.0")
			report := func(context.Context, publish.PendingReq) error {
				return test.err
			}
			handler := (&v2BackendRoute).Handler(c, report)

			handler.ServeHTTP(w, req)

			assert.Equal(t, test.code, w.Code, w.Body.String())
			approveResultBody(t, "TestReportingProblemsIntegration"+test.name, w.Body)

		})
	}
}
