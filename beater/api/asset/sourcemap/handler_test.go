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

package sourcemap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/apm-server/tests/loader"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/publish"
)

func TestAssetHandler(t *testing.T) {

	testcases := map[string]testcaseT{
		"method": {
			r:    httptest.NewRequest(http.MethodGet, "/", nil),
			code: http.StatusMethodNotAllowed,
			body: beatertest.ResultErrWrap(request.MapResultIDToStatus[request.IDResponseErrorsMethodNotAllowed].Keyword),
		},
		"large": {
			dec: func(r *http.Request) (map[string]interface{}, error) {
				return nil, errors.New("error decoding request body too large")
			},
			code: http.StatusRequestEntityTooLarge,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: error decoding request body too large",
				request.MapResultIDToStatus[request.IDResponseErrorsRequestTooLarge].Keyword)),
		},
		"decode": {
			dec: func(r *http.Request) (map[string]interface{}, error) {
				return nil, errors.New("foo")
			},
			code: http.StatusBadRequest,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: foo",
				request.MapResultIDToStatus[request.IDResponseErrorsDecode].Keyword)),
		},
		"validate": {
			dec:  func(req *http.Request) (map[string]interface{}, error) { return nil, nil },
			code: http.StatusBadRequest,
			body: "{\"error\":\"data validation error: missing properties: \\\"sourcemap\\\", expected sourcemap to be sent as string, but got null\"}\n",
		},
		"processorDecode": {
			dec: func(*http.Request) (map[string]interface{}, error) {
				return map[string]interface{}{"sourcemap": "xyz"}, nil
			},
			code: http.StatusBadRequest,
			body: "{\"error\":\"data validation error: error validating sourcemap: invalid character 'x' looking for beginning of value\"}\n",
		},
		"shuttingDown": {
			reporter: func(ctx context.Context, p publish.PendingReq) error {
				return publish.ErrChannelClosed
			},
			code: http.StatusServiceUnavailable,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: %s",
				request.MapResultIDToStatus[request.IDResponseErrorsShuttingDown].Keyword, publish.ErrChannelClosed)),
		},
		"queue": {
			reporter: func(ctx context.Context, p publish.PendingReq) error {
				return errors.New("500")
			},
			code: http.StatusServiceUnavailable,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: 500", request.MapResultIDToStatus[request.IDResponseErrorsFullQueue].Keyword)),
		},
		"valid": {
			code: http.StatusAccepted,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tc.setup()

			// test assertion
			assert.Equal(t, tc.code, tc.w.Code)
			assert.Equal(t, tc.body, tc.w.Body.String())

		})
	}
}

type testcaseT struct {
	w        *httptest.ResponseRecorder
	r        *http.Request
	dec      decoder.ReqDecoder
	reporter func(ctx context.Context, p publish.PendingReq) error

	code int
	body string
}

func (tc *testcaseT) setup() {
	if tc.w == nil {
		tc.w = httptest.NewRecorder()
	}
	if tc.r == nil {
		tc.r = httptest.NewRequest(http.MethodPost, "/", nil)
	}
	sourcemap, _ := loader.LoadDataAsBytes("../testdata/sourcemap/bundle.js.map")

	if tc.dec == nil {
		tc.dec = func(*http.Request) (map[string]interface{}, error) {
			return map[string]interface{}{
				"sourcemap":       string(sourcemap),
				"bundle_filepath": "path",
				"service_name":    "service",
				"service_version": "2",
			}, nil
		}
	}

	if tc.reporter == nil {
		tc.reporter = beatertest.NilReporter
	}
	c := &request.Context{}
	c.Reset(tc.w, tc.r)
	h := Handler(tc.dec, nil, tc.reporter)
	h(c)
}
