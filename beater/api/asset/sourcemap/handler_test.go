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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/publish"
)

type notifier struct {
	notified bool
}

func (n *notifier) NotifyAdded(ctx context.Context, serviceName, serviceVersion, bundleFilepath string) {
	n.notified = true
}

func TestAssetHandler(t *testing.T) {
	testcases := map[string]testcaseT{
		"method": {
			r:    httptest.NewRequest(http.MethodGet, "/", nil),
			code: http.StatusMethodNotAllowed,
			body: beatertest.ResultErrWrap(request.MapResultIDToStatus[request.IDResponseErrorsMethodNotAllowed].Keyword),
		},
		"decode": {
			contentType: "invalid",
			code:        http.StatusBadRequest,
			body: beatertest.ResultErrWrap(fmt.Sprintf("%s: invalid content type: invalid",
				request.MapResultIDToStatus[request.IDResponseErrorsDecode].Keyword)),
		},
		"validate": {
			missingServiceName: true,
			code:               http.StatusBadRequest,
			body:               beatertest.ResultErrWrap(fmt.Sprintf("%s: error validating sourcemap: bundle_filepath, service_name and service_version must be sent", request.MapResultIDToStatus[request.IDResponseErrorsValidate].Keyword)),
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
		"valid-full-payload": {
			sourcemapInput: func() string {
				b, err := ioutil.ReadFile("../../../../testdata/sourcemap/bundle.js.map")
				require.NoError(t, err)
				return string(b)
			}(),
			reporter: func(ctx context.Context, p publish.PendingReq) error {
				events := p.Transformable.Transform(ctx)
				docs := beatertest.EncodeEventDocs(events...)
				name := filepath.Join("test_approved", "TestProcessSourcemap")
				approvaltest.ApproveEventDocs(t, name, docs, "@timestamp")
				return nil
			},
			code: http.StatusAccepted,
		},
		"unauthorized": {
			authorizer: func(context.Context, auth.Action, auth.Resource) error {
				return auth.ErrUnauthorized
			},
			code: http.StatusForbidden,
			body: beatertest.ResultErrWrap("unauthorized"),
		},
		"auth_unavailable": {
			authorizer: func(context.Context, auth.Action, auth.Resource) error {
				return errors.New("boom")
			},
			code: http.StatusServiceUnavailable,
			body: beatertest.ResultErrWrap("service unavailable"),
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, tc.setup())
			// test assertion
			assert.Equal(t, tc.code, tc.w.Code)
			assert.Equal(t, tc.body, tc.w.Body.String())
			assert.Equal(t, tc.code == http.StatusAccepted, tc.notifier.notified)
		})
	}
}

type testcaseT struct {
	w              *httptest.ResponseRecorder
	r              *http.Request
	sourcemapInput string
	contentType    string
	reporter       func(ctx context.Context, p publish.PendingReq) error
	authorizer     authorizerFunc
	notifier       notifier

	missingSourcemap, missingServiceName, missingServiceVersion, missingBundleFilepath bool

	code int
	body string
}

func (tc *testcaseT) setup() error {
	if tc.w == nil {
		tc.w = httptest.NewRecorder()
	}
	if tc.authorizer == nil {
		tc.authorizer = func(ctx context.Context, action auth.Action, resource auth.Resource) error {
			return nil
		}
	}
	if tc.r == nil {
		buf := bytes.Buffer{}
		w := multipart.NewWriter(&buf)
		if !tc.missingSourcemap {
			part, err := w.CreateFormFile("sourcemap", "bundle_no_mapping.js.map")
			if err != nil {
				return err
			}
			if tc.sourcemapInput == "" {
				tc.sourcemapInput = "sourcemap dummy string"
			}
			if _, err = io.Copy(part, strings.NewReader(tc.sourcemapInput)); err != nil {
				return err
			}
		}
		if !tc.missingBundleFilepath {
			w.WriteField("bundle_filepath", "js/./test/../bundle_no_mapping.js.map")
		}
		if !tc.missingServiceName {
			w.WriteField("service_name", "My service")
		}
		if !tc.missingServiceVersion {
			w.WriteField("service_version", "0.1")
		}
		if err := w.Close(); err != nil {
			return err
		}
		tc.r = httptest.NewRequest(http.MethodPost, "/", &buf)
		if tc.contentType == "" {
			tc.contentType = w.FormDataContentType()
		}
		tc.r.Header.Set("Content-Type", tc.contentType)
		tc.r = tc.r.WithContext(auth.ContextWithAuthorizer(tc.r.Context(), tc.authorizer))
	}

	if tc.reporter == nil {
		tc.reporter = func(context.Context, publish.PendingReq) error { return nil }
	}
	c := request.NewContext()
	c.Reset(tc.w, tc.r)
	h := Handler(tc.reporter, &tc.notifier)
	h(c)
	return nil
}

type authorizerFunc func(context.Context, auth.Action, auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}
