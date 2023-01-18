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

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/kibana"
)

var (
	mockEtag    = "1c9588f5a4da71cdef992981a9c9735c"
	successBody = map[string]string{"sampling_rate": "0.5"}
	emptyBody   = map[string]string{}

	testcases = map[string]struct {
		fetchResult agentcfg.Result
		fetchErr    error

		requestHeader                          map[string]string
		queryParams                            map[string]string
		method                                 string
		respStatus                             int
		respBody                               map[string]string
		respEtagHeader, respCacheControlHeader string
	}{
		"NotModified": {
			fetchResult: agentcfg.Result{
				Source: agentcfg.Source{
					Etag:     mockEtag,
					Settings: map[string]string{"sampling_rate": "0.5"},
				},
			},
			method:                 http.MethodGet,
			requestHeader:          map[string]string{headers.IfNoneMatch: `"` + mockEtag + `"`},
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusNotModified,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respEtagHeader:         `"` + mockEtag + `"`,
		},

		"ModifiedWithEtag": {
			fetchResult: agentcfg.Result{
				Source: agentcfg.Source{
					Etag:     mockEtag,
					Settings: map[string]string{"sampling_rate": "0.5"},
				},
			},
			method:                 http.MethodGet,
			requestHeader:          map[string]string{headers.IfNoneMatch: "2"},
			queryParams:            map[string]string{"service.name": "opbeans-java"},
			respStatus:             http.StatusOK,
			respEtagHeader:         `"` + mockEtag + `"`,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respBody:               successBody,
		},

		"NoConfigFound": {
			fetchResult: agentcfg.Result{
				Source: agentcfg.Source{
					Etag:     agentcfg.EtagSentinel,
					Settings: map[string]string{},
				},
			},
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-python"},
			respStatus:             http.StatusOK,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respEtagHeader:         fmt.Sprintf("\"%s\"", agentcfg.EtagSentinel),
			respBody:               emptyBody,
		},

		"SendToKibanaFailed": {
			fetchErr:               errors.New("fetch failed"),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": "fetch failed"},
		},

		"NoService": {
			method:                 http.MethodGet,
			respStatus:             http.StatusBadRequest,
			respBody:               map[string]string{"error": "service.name is required"},
			respCacheControlHeader: "max-age=300, must-revalidate",
		},

		"MethodNotAllowed": {
			method:                 http.MethodPut,
			respStatus:             http.StatusMethodNotAllowed,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": fmt.Sprintf("%s: PUT", msgMethodUnsupported)},
		},

		"Unauthorized": {
			fetchErr:               errors.New("Unauthorized"),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody: map[string]string{"error": "APM Server is not authorized to query Kibana. " +
				"Please configure apm-server.kibana.username and apm-server.kibana.password, " +
				"and ensure the user has the necessary privileges."},
		},
	}
)

func TestAgentConfigHandler(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			var fetcher fetcherFunc = func(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
				return tc.fetchResult, tc.fetchErr
			}
			h := NewHandler(fetcher, 4*time.Second, "", nil)
			r := httptest.NewRequest(tc.method, target(tc.queryParams), nil)
			for k, v := range tc.requestHeader {
				r.Header.Set(k, v)
			}
			ctx, w := newRequestContext(r)
			h(ctx)

			require.Equal(t, tc.respStatus, w.Code)
			require.Equal(t, tc.respCacheControlHeader, w.Header().Get(headers.CacheControl))
			require.Equal(t, tc.respEtagHeader, w.Header().Get(headers.Etag))
			b, err := io.ReadAll(w.Body)
			require.NoError(t, err)
			var actualBody map[string]string
			json.Unmarshal(b, &actualBody)
			assert.Equal(t, tc.respBody, actualBody)
		})
	}
}

func TestAgentConfigHandlerAnonymousAccess(t *testing.T) {
	var fetcher fetcherFunc = func(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
		return agentcfg.Result{}, errors.New("Unauthorized")
	}
	h := NewHandler(fetcher, time.Nanosecond, "", nil)

	for _, tc := range []struct {
		anonymous    bool
		response     string
		authResource *auth.Resource
	}{{
		anonymous:    false,
		response:     `{"error":"APM Server is not authorized to query Kibana. Please configure apm-server.kibana.username and apm-server.kibana.password, and ensure the user has the necessary privileges."}`,
		authResource: &auth.Resource{ServiceName: "opbeans"},
	}, {
		anonymous:    true,
		response:     `{"error":"Unauthorized"}`,
		authResource: &auth.Resource{ServiceName: "opbeans"},
	}} {
		r := httptest.NewRequest(http.MethodGet, target(map[string]string{"service.name": "opbeans"}), nil)
		c, w := newRequestContext(r)

		c.Authentication.Method = "none"
		if tc.anonymous {
			c.Authentication.Method = ""
		}

		var requestedResource *auth.Resource
		c.Request = withAuthorizer(c.Request,
			authorizerFunc(func(ctx context.Context, action auth.Action, resource auth.Resource) error {
				if requestedResource != nil {
					panic("expected only one Authorize request")
				}
				requestedResource = &resource
				return nil
			}),
		)
		h(c)
		assert.Equal(t, tc.response+"\n", w.Body.String())
		assert.Equal(t, tc.authResource, requestedResource)
	}
}

func TestAgentConfigHandlerAuthorizedForService(t *testing.T) {
	var called bool
	f := newSanitizingKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	h := NewHandler(f, time.Nanosecond, "", nil)

	r := httptest.NewRequest(http.MethodGet, target(map[string]string{"service.name": "opbeans"}), nil)
	ctx, w := newRequestContext(r)

	var queriedResource auth.Resource
	ctx.Request = withAuthorizer(ctx.Request,
		authorizerFunc(func(ctx context.Context, action auth.Action, resource auth.Resource) error {
			queriedResource = resource
			return auth.ErrUnauthorized
		}),
	)
	h(ctx)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, auth.Resource{ServiceName: "opbeans"}, queriedResource)
	assert.False(t, called)
}

func TestConfigAgentHandler_DirectConfiguration(t *testing.T) {
	fetcher := agentcfg.NewDirectFetcher([]agentcfg.AgentConfig{{
		ServiceName: "service1",
		Config:      map[string]string{"key1": "val1"},
		Etag:        "abc123",
	}})
	h := NewHandler(fetcher, time.Nanosecond, "", nil)

	w := sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", jsonReader(map[string]interface{}{
		"service": map[string]interface{}{
			"name": "service1",
		},
	})))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"key1":"val1"}`, w.Body.String())
}

func TestAgentConfigHandler_PostOk(t *testing.T) {
	f := newSanitizingKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"_id": "1", "_source": {"settings": {"sampling_rate": 0.5}}}`)
	})
	h := NewHandler(f, time.Nanosecond, "", nil)

	w := sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", jsonReader(map[string]interface{}{
		"service": map[string]interface{}{
			"name": "opbeans-node",
		},
	})))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestAgentConfigHandler_DefaultServiceEnvironment(t *testing.T) {
	var requestBodies []string
	f := newSanitizingKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBodies = append(requestBodies, string(body))
		fmt.Fprintln(w, `{"_id": "1", "_source": {"settings": {"sampling_rate": 0.5}}}`)
	})
	h := NewHandler(f, time.Nanosecond, "default", nil)

	sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", jsonReader(map[string]interface{}{"service": map[string]interface{}{"name": "opbeans-node", "environment": "specified"}})))
	sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", jsonReader(map[string]interface{}{"service": map[string]interface{}{"name": "opbeans-node"}})))

	assert.Equal(t, []string{
		`{"service":{"name":"opbeans-node","environment":"specified"},"etag":""}` + "\n",
		`{"service":{"name":"opbeans-node","environment":"default"},"etag":""}` + "\n",
	}, requestBodies)
}

func TestAgentConfigRum(t *testing.T) {
	h := getHandler(t, "rum-js")
	r := httptest.NewRequest(http.MethodPost, "/rum", jsonReader(map[string]interface{}{
		"service": map[string]interface{}{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.Authentication.Method = "" // unauthenticated
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, headers.Etag, w.Header().Get(headers.AccessControlExposeHeaders))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{"transaction_sample_rate": "0.5"}, actual)
}

func TestAgentConfigRumEtag(t *testing.T) {
	h := getHandler(t, "rum-js")
	r := httptest.NewRequest(http.MethodGet, "/rum?ifnonematch=123&service.name=opbeans", nil)
	ctx, w := newRequestContext(r)
	h(ctx)
	assert.Equal(t, http.StatusNotModified, w.Code, w.Body.String())
}

func TestAgentConfigNotRum(t *testing.T) {
	h := getHandler(t, "node-js")
	r := httptest.NewRequest(http.MethodPost, "/backend", jsonReader(map[string]interface{}{
		"service": map[string]interface{}{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.Request = withAuthorizer(ctx.Request,
		authorizerFunc(func(context.Context, auth.Action, auth.Resource) error {
			return nil
		}),
	)
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{"capture_body": "transactions", "transaction_sample_rate": "0.5"}, actual)
}

func TestAgentConfigNoLeak(t *testing.T) {
	h := getHandler(t, "node-js")
	r := httptest.NewRequest(http.MethodPost, "/rum", jsonReader(map[string]interface{}{
		"service": map[string]interface{}{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.Authentication.Method = "" // unauthenticated
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{}, actual)
}

func getHandler(t testing.TB, agent string) request.Handler {
	f := newSanitizingKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"_id": "1",
			"_source": map[string]interface{}{
				"settings": map[string]interface{}{
					"transaction_sample_rate": 0.5,
					"capture_body":            "transactions",
				},
				"etag":       "123",
				"agent_name": agent,
			},
		})
	})
	return NewHandler(f, time.Nanosecond, "", []string{"rum-js"})
}

func TestIfNoneMatch(t *testing.T) {
	var fromHeader = func(s string) *request.Context {
		r := &http.Request{Header: map[string][]string{"If-None-Match": {s}}}
		return &request.Context{Request: r}
	}

	var fromQueryArg = func(s string) *request.Context {
		r := &http.Request{}
		r.URL, _ = url.Parse("http://host:8200/path?ifnonematch=123")
		return &request.Context{Request: r}
	}

	assert.Equal(t, "123", ifNoneMatch(fromHeader("123")))
	assert.Equal(t, "123", ifNoneMatch(fromHeader(`"123"`)))
	assert.Equal(t, "123", ifNoneMatch(fromQueryArg("123")))
}

func TestAgentConfigContext(t *testing.T) {
	// The request context should be passed to Fetch.
	type contextKey struct{}
	var contextValue interface{}
	var fetcher fetcherFunc = func(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
		contextValue = ctx.Value(contextKey{})
		return agentcfg.Result{}, nil
	}
	handler := NewHandler(fetcher, 5*time.Minute, "default", nil)
	r := httptest.NewRequest("GET", target(map[string]string{"service.name": "opbeans"}), nil)
	r = r.WithContext(context.WithValue(r.Context(), contextKey{}, "value"))
	c, _ := newRequestContext(r)
	handler(c)
	assert.Equal(t, "value", contextValue)
}

func sendRequest(h request.Handler, r *http.Request) *httptest.ResponseRecorder {
	ctx, recorder := newRequestContext(r)
	ctx.Request = withAuthorizer(ctx.Request,
		authorizerFunc(func(context.Context, auth.Action, auth.Resource) error {
			return nil
		}),
	)
	h(ctx)
	return recorder
}

func newRequestContext(r *http.Request) (*request.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx := request.NewContext()
	ctx.Reset(w, r)
	ctx.Request = withAnonymousAuthorizer(ctx.Request)
	ctx.Authentication.Method = auth.MethodNone
	return ctx, w
}

func target(params map[string]string) string {
	t := "/config"
	if len(params) == 0 {
		return t
	}
	t += "?"
	for k, v := range params {
		t = fmt.Sprintf("%s%s=%s", t, k, v)
	}
	return t
}

func newSanitizingKibanaFetcher(t testing.TB, h http.HandlerFunc) agentcfg.Fetcher {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client, err := kibana.NewClient(kibana.ClientConfig{Host: srv.URL})
	require.NoError(t, err)
	kf, err := agentcfg.NewKibanaFetcher(client, time.Nanosecond)
	require.NoError(t, err)
	return agentcfg.SanitizingFetcher{Fetcher: kf}
}

type fetcherFunc func(context.Context, agentcfg.Query) (agentcfg.Result, error)

func (f fetcherFunc) Fetch(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
	return f(ctx, query)
}

func withAnonymousAuthorizer(req *http.Request) *http.Request {
	return withAuthorizer(req, authorizerFunc(func(context.Context, auth.Action, auth.Resource) error {
		return nil
	}))
}

func withAuthorizer(req *http.Request, authz auth.Authorizer) *http.Request {
	return req.WithContext(auth.ContextWithAuthorizer(req.Context(), authz))
}

type authorizerFunc func(context.Context, auth.Action, auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}

func jsonReader(v interface{}) io.Reader {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}
