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

package intake

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
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/approvals"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
)

func TestIntakeHandler(t *testing.T) {

	for name, tc := range map[string]testcaseIntakeHandler{
		"Method": {
			path: "errors.ndjson",
			r:    httptest.NewRequest(http.MethodGet, "/", nil),
			code: http.StatusBadRequest, id: request.IDResponseErrorsMethodNotAllowed,
		},
		"ContentType": {
			path: "errors.ndjson",
			r: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set(headers.ContentType, "application/json")
				return req
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"RateLimit": {
			path: "errors.ndjson",
			rlc:  &mockBlockingRateLimiter{},
			code: http.StatusTooManyRequests, id: request.IDResponseErrorsRateLimit,
		},
		"BodyReader": {
			path: "errors.ndjson",
			r: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set(headers.ContentType, "application/x-ndjson")
				return req
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"CompressedBodyReaderDeflate": {
			path: "errors.ndjson",
			r: func() *http.Request {
				data, err := loader.LoadDataAsBytes("../testdata/intake-v2/errors.ndjson")
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(data))
				req.Header.Set(headers.ContentType, "application/x-ndjson")
				req.Header.Set(headers.ContentEncoding, "deflate")
				return req
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"CompressedBodyReaderGzip": {
			path: "errors.ndjson",
			r: func() *http.Request {
				data, err := loader.LoadDataAsBytes("../testdata/intake-v2/errors.ndjson")
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(data))
				req.Header.Set(headers.ContentType, "application/x-ndjson")
				req.Header.Set(headers.ContentEncoding, "gzip")
				return req
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"Decoder": {
			path: "errors.ndjson",
			dec: func(*http.Request) (map[string]interface{}, error) {
				return nil, errors.New("cannot decode `xyz`")
			},
			code: http.StatusInternalServerError, id: request.IDResponseErrorsInternal,
		},
		"TooLarge": {
			path: "errors.ndjson", processor: &stream.Processor{},
			code: http.StatusBadRequest, id: request.IDResponseErrorsRequestTooLarge},
		"Closing": {
			path: "errors.ndjson", reporter: beatertest.ErrorReporterFn(publish.ErrChannelClosed),
			code: http.StatusServiceUnavailable, id: request.IDResponseErrorsShuttingDown},
		"FullQueue": {
			path: "errors.ndjson", reporter: beatertest.ErrorReporterFn(publish.ErrFull),
			code: http.StatusServiceUnavailable, id: request.IDResponseErrorsFullQueue},
		"InvalidEvent": {
			path: "invalid-event.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"InvalidJSONEvent": {
			path: "invalid-json-event.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"InvalidJSONMetadata": {
			path: "invalid-json-metadata.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"InvalidMetadata": {
			path: "invalid-metadata.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"InvalidMetadata2": {
			path: "invalid-metadata-2.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"UnrecognizedEvent": {
			path: "unrecognized-event.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"Success": {
			path: "errors.ndjson",
			code: http.StatusAccepted, id: request.IDResponseValidAccepted},
	} {
		// setup
		tc.setup(t)

		// call handler
		h := Handler(tc.dec, tc.processor, tc.rlc, tc.reporter)
		h(tc.c)

		t.Run(name+"ID", func(t *testing.T) {
			assert.Equal(t, tc.id, tc.c.Result.ID)
		})
		t.Run(name+"Response", func(t *testing.T) {
			assert.Equal(t, tc.code, tc.w.Code)
			assert.Equal(t, "application/json", tc.w.Header().Get(headers.ContentType))

			if tc.code == http.StatusAccepted {
				assert.NotNil(t, tc.w.Body.Len())
			}
			body := tc.w.Body.Bytes()
			approvals.AssertApproveResult(t, "test_approved/"+name, body)
		})
	}
}

type testcaseIntakeHandler struct {
	c         *request.Context
	w         *httptest.ResponseRecorder
	r         *http.Request
	dec       decoder.ReqDecoder
	processor *stream.Processor
	rlc       RateLimiterManager
	reporter  func(ctx context.Context, p publish.PendingReq) error
	path      string

	code int
	id   request.ResultID
}

func (tc *testcaseIntakeHandler) setup(t *testing.T) {
	if tc.dec == nil {
		tc.dec = emptyDec
	}
	if tc.processor == nil {
		cfg := config.DefaultConfig("7.0.0")
		tc.processor = &stream.Processor{
			Tconfig:      transform.Config{},
			Mconfig:      model.Config{Experimental: cfg.Mode == config.ModeExperimental},
			MaxEventSize: cfg.MaxEventSize,
		}
	}
	if tc.reporter == nil {
		tc.reporter = beatertest.NilReporter
	}

	if tc.r == nil {
		data, err := loader.LoadDataAsBytes(filepath.Join("../testdata/intake-v2/", tc.path))
		require.NoError(t, err)

		tc.r = httptest.NewRequest("POST", "/", bytes.NewBuffer(data))
		tc.r.Header.Add("Content-Type", "application/x-ndjson")
		q := tc.r.URL.Query()
		q.Add("verbose", "")
		tc.r.URL.RawQuery = q.Encode()
	}
	tc.r.Header.Add("Accept", "application/json")

	tc.w = httptest.NewRecorder()
	tc.c = &request.Context{}
	tc.c.Reset(tc.w, tc.r)
}

type mockBlockingRateLimiter struct{}

func (m *mockBlockingRateLimiter) RateLimiter(key string) (*rate.Limiter, bool) {
	return rate.NewLimiter(rate.Limit(0), 0), true
}

func emptyDec(*http.Request) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
