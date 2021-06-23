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
	"compress/gzip"
	"compress/zlib"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/api/ratelimit"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/publish"
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
		"BodyReader": {
			path: "errors.ndjson",
			r: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set(headers.ContentType, "application/x-ndjson")
				return req
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"CompressedBodyReaderDeflateInvalid": {
			path: "errors.ndjson",
			r:    compressedRequest(t, "deflate", false),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"CompressedBodyReaderDeflateValid": {
			path: "errors.ndjson",
			r:    compressedRequest(t, "deflate", true),
			code: http.StatusAccepted, id: request.IDResponseValidAccepted,
		},
		"CompressedBodyReaderGzipInvalid": {
			path: "errors.ndjson",
			r:    compressedRequest(t, "gzip", false),
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate,
		},
		"CompressedBodyReaderGzipValid": {
			path: "errors.ndjson",
			r:    compressedRequest(t, "gzip", true),
			code: http.StatusAccepted, id: request.IDResponseValidAccepted,
		},
		"TooLarge": {
			path: "errors.ndjson",
			processor: func() *stream.Processor {
				p := stream.BackendProcessor(config.DefaultConfig())
				p.MaxEventSize = 10
				return p
			}(),
			code: http.StatusBadRequest, id: request.IDResponseErrorsRequestTooLarge},
		"Closing": {
			path: "errors.ndjson",
			batchProcessor: model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
				return publish.ErrChannelClosed
			}),
			code: http.StatusServiceUnavailable, id: request.IDResponseErrorsShuttingDown},
		"FullQueue": {
			path: "errors.ndjson",
			batchProcessor: model.ProcessBatchFunc(func(context.Context, *model.Batch) error {
				return publish.ErrFull
			}),
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
			path: "invalid-event-type.ndjson",
			code: http.StatusBadRequest, id: request.IDResponseErrorsValidate},
		"Success": {
			path: "errors.ndjson",
			code: http.StatusAccepted, id: request.IDResponseValidAccepted},
	} {
		t.Run(name, func(t *testing.T) {

			// setup
			tc.setup(t)

			// call handler
			h := Handler(tc.processor, emptyRequestMetadata, tc.batchProcessor)
			h(tc.c)

			require.Equal(t, string(tc.id), string(tc.c.Result.ID))
			assert.Equal(t, tc.code, tc.w.Code)
			assert.Equal(t, "application/json", tc.w.Header().Get(headers.ContentType))

			if tc.code == http.StatusAccepted {
				assert.NotNil(t, tc.w.Body.Len())
				assert.Nil(t, tc.c.Result.Err)
			} else {
				assert.NotNil(t, tc.c.Result.Err)
			}
			body := tc.w.Body.Bytes()
			approvaltest.ApproveJSON(t, "test_approved/"+name, body)
		})
	}
}

func TestRateLimiting(t *testing.T) {
	type test struct {
		limiter       *rate.Limiter
		preconsumed   int
		expectLimited bool
	}

	for name, test := range map[string]test{
		"LimiterAllowAll": {
			limiter:       rate.NewLimiter(rate.Limit(40), 40*5),
			expectLimited: false,
		},
		"LimiterPartiallyUsedLimitAllow": {
			limiter:       rate.NewLimiter(rate.Limit(10), 10*2),
			preconsumed:   10,
			expectLimited: false,
		},
		"LimiterDenyAll": {
			limiter:       rate.NewLimiter(rate.Limit(0), 2),
			expectLimited: true,
		},
		"LimiterPartiallyUsedLimitDeny": {
			limiter:       rate.NewLimiter(rate.Limit(7), 7*2),
			preconsumed:   10,
			expectLimited: true,
		},
		"LimiterDeny": {
			limiter:       rate.NewLimiter(rate.Limit(6), 6*2),
			expectLimited: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var tc testcaseIntakeHandler
			tc.path = "ratelimit.ndjson"
			tc.setup(t)

			tc.c.Request = tc.c.Request.WithContext(
				ratelimit.ContextWithLimiter(tc.c.Request.Context(), test.limiter),
			)
			if test.preconsumed > 0 {
				test.limiter.AllowN(time.Now(), test.preconsumed)
			}

			h := Handler(tc.processor, emptyRequestMetadata, tc.batchProcessor)
			h(tc.c)

			if test.expectLimited {
				assert.Equal(t, request.IDResponseErrorsRateLimit, tc.c.Result.ID)
				assert.Equal(t, http.StatusTooManyRequests, tc.w.Code)
				assert.Error(t, tc.c.Result.Err)
			} else {
				assert.Equal(t, request.IDResponseValidAccepted, tc.c.Result.ID)
				assert.Equal(t, http.StatusAccepted, tc.w.Code)
				assert.NoError(t, tc.c.Result.Err)
			}
			assert.NotZero(t, tc.w.Body.Len())
			approvaltest.ApproveJSON(t, "test_approved/"+t.Name(), tc.w.Body.Bytes())
		})
	}
}

type testcaseIntakeHandler struct {
	c              *request.Context
	w              *httptest.ResponseRecorder
	r              *http.Request
	processor      *stream.Processor
	batchProcessor model.BatchProcessor
	path           string

	code int
	id   request.ResultID
}

func (tc *testcaseIntakeHandler) setup(t *testing.T) {
	if tc.processor == nil {
		cfg := config.DefaultConfig()
		tc.processor = stream.BackendProcessor(cfg)
	}
	if tc.batchProcessor == nil {
		tc.batchProcessor = modelprocessor.Nop{}
	}

	if tc.r == nil {
		data, err := ioutil.ReadFile(filepath.Join("../../../testdata/intake-v2", tc.path))
		require.NoError(t, err)

		tc.r = httptest.NewRequest("POST", "/", bytes.NewBuffer(data))
		tc.r.Header.Add("Content-Type", "application/x-ndjson")
	}
	q := tc.r.URL.Query()
	q.Add("verbose", "")
	tc.r.URL.RawQuery = q.Encode()
	tc.r.Header.Add("Accept", "application/json")

	tc.w = httptest.NewRecorder()
	tc.c = request.NewContext()
	tc.c.Reset(tc.w, tc.r)
}

func compressedRequest(t *testing.T, compressionType string, compressPayload bool) *http.Request {
	data, err := ioutil.ReadFile("../../../testdata/intake-v2/errors.ndjson")
	require.NoError(t, err)
	var buf bytes.Buffer
	if compressPayload {
		switch compressionType {
		case "gzip":
			w := gzip.NewWriter(&buf)
			_, err = w.Write(data)
			require.NoError(t, w.Close())
		case "deflate":
			w := zlib.NewWriter(&buf)
			_, err = w.Write(data)
			require.NoError(t, w.Close())
		}
	} else {
		_, err = buf.Write(data)
	}
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set(headers.ContentType, "application/x-ndjson")
	req.Header.Set(headers.ContentEncoding, compressionType)
	return req
}

func emptyRequestMetadata(*request.Context) model.Metadata {
	return model.Metadata{}
}
