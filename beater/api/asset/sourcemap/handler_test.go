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
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/asset"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
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
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: no input", request.MapResultIDToStatus[request.IDResponseErrorsValidate].Keyword)),
		},
		"processorDecode": {
			dec: func(*http.Request) (map[string]interface{}, error) {
				return map[string]interface{}{"mockProcessor": "xyz"}, nil
			},
			code: http.StatusBadRequest,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: processor decode error", request.MapResultIDToStatus[request.IDResponseErrorsDecode].Keyword)),
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
	w         *httptest.ResponseRecorder
	r         *http.Request
	dec       RequestDecoder
	processor asset.Processor
	reporter  func(ctx context.Context, p publish.PendingReq) error

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
	if tc.dec == nil {
		tc.dec = func(*http.Request) (map[string]interface{}, error) {
			return map[string]interface{}{"foo": "bar"}, nil
		}
	}
	if tc.processor == nil {
		tc.processor = &mockProcessor{}
	}
	if tc.reporter == nil {
		tc.reporter = beatertest.NilReporter
	}
	c := request.NewContext()
	c.Reset(tc.w, tc.r)
	h := Handler(tc.dec, tc.processor, tc.reporter)
	h(c)
}

type mockProcessor struct{}

func (p *mockProcessor) Validate(m map[string]interface{}) error {
	if m == nil {
		return errors.New("no input")
	}
	return nil
}
func (p *mockProcessor) Decode(m map[string]interface{}) ([]transform.Transformable, error) {
	if _, ok := m["mockProcessor"]; ok {
		return nil, errors.New("processor decode error")
	}
	return nil, nil
}
func (p *mockProcessor) Name() string {
	return "mockProcessor"
}

func TestDecodeSourcemapFormData(t *testing.T) {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileBytes, err := loader.LoadDataAsBytes("../testdata/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	part, err := writer.CreateFormFile("sourcemap", "bundle_no_mapping.js.map")
	assert.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(fileBytes))
	assert.NoError(t, err)

	writer.WriteField("bundle_filepath", "js/./test/../bundle_no_mapping.js.map")
	writer.WriteField("service_name", "My service")
	writer.WriteField("service_version", "0.1")

	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest("POST", "_", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	assert.NoError(t, err)

	assert.NoError(t, err)
	data, err := DecodeSourcemapFormData(req)
	assert.NoError(t, err)

	assert.Len(t, data, 4)
	assert.Equal(t, "js/bundle_no_mapping.js.map", data["bundle_filepath"])
	assert.Equal(t, "My service", data["service_name"])
	assert.Equal(t, "0.1", data["service_version"])
	assert.NotNil(t, data["sourcemap"].(string))
	assert.Equal(t, len(fileBytes), len(data["sourcemap"].(string)))
}
