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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/apmtest"
	"golang.org/x/time/rate"

	"github.com/elastic/beats/v7/libbeat/common"
	libkibana "github.com/elastic/beats/v7/libbeat/kibana"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/tests"
)

type m map[string]interface{}

var (
	mockVersion = *common.MustNewVersion("7.5.0")
	mockEtag    = "1c9588f5a4da71cdef992981a9c9735c"
	successBody = map[string]string{"sampling_rate": "0.5"}
	emptyBody   = map[string]string{}

	testcases = map[string]struct {
		kbClient                               kibana.Client
		requestHeader                          map[string]string
		queryParams                            map[string]string
		method                                 string
		respStatus                             int
		respBodyToken                          map[string]string
		respBody                               map[string]string
		respEtagHeader, respCacheControlHeader string
	}{
		"NotModified": {
			kbClient: tests.MockKibana(http.StatusOK, m{
				"_id": "1",
				"_source": m{
					"settings": m{
						"sampling_rate": 0.5,
					},
					"etag": mockEtag,
				},
			}, mockVersion, true),
			method:                 http.MethodGet,
			requestHeader:          map[string]string{headers.IfNoneMatch: `"` + mockEtag + `"`},
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusNotModified,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respEtagHeader:         `"` + mockEtag + `"`,
		},

		"ModifiedWithEtag": {
			kbClient: tests.MockKibana(http.StatusOK, m{
				"_id": "1",
				"_source": m{
					"settings": m{
						"sampling_rate": 0.5,
					},
					"etag": mockEtag,
				},
			}, mockVersion, true),
			method:                 http.MethodGet,
			requestHeader:          map[string]string{headers.IfNoneMatch: "2"},
			queryParams:            map[string]string{"service.name": "opbeans-java"},
			respStatus:             http.StatusOK,
			respEtagHeader:         `"` + mockEtag + `"`,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respBody:               successBody,
			respBodyToken:          successBody,
		},

		"NoConfigFound": {
			kbClient:               tests.MockKibana(http.StatusNotFound, m{}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-python"},
			respStatus:             http.StatusOK,
			respCacheControlHeader: "max-age=4, must-revalidate",
			respEtagHeader:         fmt.Sprintf("\"%s\"", agentcfg.EtagSentinel),
			respBody:               emptyBody,
			respBodyToken:          emptyBody,
		},

		"SendToKibanaFailed": {
			kbClient:               tests.MockKibana(http.StatusBadGateway, m{}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": agentcfg.ErrMsgSendToKibanaFailed},
			respBodyToken:          map[string]string{"error": fmt.Sprintf("%s: testerror", agentcfg.ErrMsgSendToKibanaFailed)},
		},

		"NoConnection": {
			kbClient:               tests.MockKibana(http.StatusServiceUnavailable, m{}, mockVersion, false),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": agentcfg.ErrMsgNoKibanaConnection},
			respBodyToken:          map[string]string{"error": agentcfg.ErrMsgNoKibanaConnection},
		},

		"InvalidVersion": {
			kbClient: tests.MockKibana(http.StatusServiceUnavailable, m{},
				*common.MustNewVersion("7.2.0"), true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": agentcfg.ErrMsgKibanaVersionNotCompatible},
			respBodyToken: map[string]string{"error": fmt.Sprintf("%s: min version 7.5.0, "+
				"configured version 7.2.0", agentcfg.ErrMsgKibanaVersionNotCompatible)},
		},

		"NoService": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodGet,
			respStatus:             http.StatusBadRequest,
			respBody:               map[string]string{"error": msgInvalidQuery},
			respBodyToken:          map[string]string{"error": "service.name is required"},
			respCacheControlHeader: "max-age=300, must-revalidate",
		},

		"MethodNotAllowed": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodPut,
			respStatus:             http.StatusMethodNotAllowed,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": msgMethodUnsupported},
			respBodyToken:          map[string]string{"error": fmt.Sprintf("%s: PUT", msgMethodUnsupported)},
		},

		"Unauthorized": {
			kbClient:               tests.MockKibana(http.StatusUnauthorized, m{"error": "Unauthorized"}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-node"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               map[string]string{"error": agentcfg.ErrUnauthorized},
			respBodyToken: map[string]string{"error": "APM Server is not authorized to query Kibana. " +
				"Please configure apm-server.kibana.username and apm-server.kibana.password, " +
				"and ensure the user has the necessary privileges."},
		},
	}
)

func TestAgentConfigHandler(t *testing.T) {
	var cfg = config.KibanaAgentConfig{Cache: &config.Cache{Expiration: 4 * time.Second}}

	for name, tc := range testcases {

		runTest := func(t *testing.T, expectedBody map[string]string, authorized bool) {
			f := agentcfg.NewFetcher(tc.kbClient, cfg.Cache.Expiration)
			h := NewHandler(f, &cfg, "")
			r := httptest.NewRequest(tc.method, target(tc.queryParams), nil)
			for k, v := range tc.requestHeader {
				r.Header.Set(k, v)
			}
			ctx, w := newRequestContext(r)
			ctx.AuthResult.Authorized = authorized
			h(ctx)

			require.Equal(t, tc.respStatus, w.Code)
			require.Equal(t, tc.respCacheControlHeader, w.Header().Get(headers.CacheControl))
			require.Equal(t, tc.respEtagHeader, w.Header().Get(headers.Etag))
			b, err := ioutil.ReadAll(w.Body)
			require.NoError(t, err)
			var actualBody map[string]string
			json.Unmarshal(b, &actualBody)
			assert.Equal(t, expectedBody, actualBody)
		}

		t.Run(name+"NoSecretToken", func(t *testing.T) {
			runTest(t, tc.respBody, false)
		})

		t.Run(name+"WithSecretToken", func(t *testing.T) {
			runTest(t, tc.respBodyToken, true)
		})
	}
}

func TestAgentConfigHandler_NoKibanaClient(t *testing.T) {
	cfg := config.KibanaAgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	f := agentcfg.NewFetcher(nil, cfg.Cache.Expiration)
	h := NewHandler(f, &cfg, "")

	w := sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{
		"service": m{"name": "opbeans-node"}})))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
}

func TestAgentConfigHandler_PostOk(t *testing.T) {
	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": 0.5,
			},
		},
	}, mockVersion, true)

	var cfg = config.KibanaAgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	f := agentcfg.NewFetcher(kb, cfg.Cache.Expiration)
	h := NewHandler(f, &cfg, "")

	w := sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{
		"service": m{"name": "opbeans-node"}})))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestAgentConfigHandler_DefaultServiceEnvironment(t *testing.T) {
	kb := &recordingKibanaClient{
		Client: tests.MockKibana(http.StatusOK, m{
			"_id": "1",
			"_source": m{
				"settings": m{
					"sampling_rate": 0.5,
				},
			},
		}, mockVersion, true),
	}

	var cfg = config.KibanaAgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	f := agentcfg.NewFetcher(kb, cfg.Cache.Expiration)
	h := NewHandler(f, &cfg, "default")

	sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{"service": m{"name": "opbeans-node", "environment": "specified"}})))
	sendRequest(h, httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{"service": m{"name": "opbeans-node"}})))
	require.Len(t, kb.requests, 2)

	body0, _ := ioutil.ReadAll(kb.requests[0].Body)
	body1, _ := ioutil.ReadAll(kb.requests[1].Body)
	assert.Equal(t, `{"service":{"name":"opbeans-node","environment":"specified"},"etag":""}`, string(body0))
	assert.Equal(t, `{"service":{"name":"opbeans-node","environment":"default"},"etag":""}`, string(body1))
}

func TestAgentConfigRum(t *testing.T) {
	h := getHandler("rum-js")
	r := httptest.NewRequest(http.MethodPost, "/rum", convert.ToReader(m{
		"service": m{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.IsRum = true
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, headers.Etag, w.Header().Get(headers.AccessControlExposeHeaders))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{"transaction_sample_rate": "0.5"}, actual)
}

func TestAgentConfigRumEtag(t *testing.T) {
	h := getHandler("rum-js")
	r := httptest.NewRequest(http.MethodGet, "/rum?ifnonematch=123&service.name=opbeans", nil)
	ctx, w := newRequestContext(r)
	ctx.IsRum = true
	h(ctx)
	assert.Equal(t, http.StatusNotModified, w.Code, w.Body.String())
}

func TestAgentConfigNotRum(t *testing.T) {
	h := getHandler("node-js")
	r := httptest.NewRequest(http.MethodPost, "/backend", convert.ToReader(m{
		"service": m{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{"capture_body": "transactions", "transaction_sample_rate": "0.5"}, actual)
}

func TestAgentConfigNoLeak(t *testing.T) {
	h := getHandler("node-js")
	r := httptest.NewRequest(http.MethodPost, "/rum", convert.ToReader(m{
		"service": m{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.IsRum = true
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{}, actual)
}

func TestAgentConfigRateLimit(t *testing.T) {
	h := getHandler("rum-js")
	r := httptest.NewRequest(http.MethodPost, "/rum", convert.ToReader(m{
		"service": m{"name": "opbeans"}}))
	ctx, w := newRequestContext(r)
	ctx.IsRum = true
	ctx.RateLimiter = rate.NewLimiter(rate.Limit(0), 0)
	h(ctx)
	var actual map[string]string
	json.Unmarshal(w.Body.Bytes(), &actual)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
	assert.Equal(t, map[string]string{"error": "too many requests"}, actual)
}

func getHandler(agent string) request.Handler {
	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"transaction_sample_rate": 0.5,
				"capture_body":            "transactions",
			},
			"etag":       "123",
			"agent_name": agent,
		},
	}, mockVersion, true)
	cfg := config.KibanaAgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	f := agentcfg.NewFetcher(kb, cfg.Cache.Expiration)
	return NewHandler(f, &cfg, "")
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

func TestAgentConfigTraceContext(t *testing.T) {
	kibanaCfg := config.KibanaConfig{Enabled: true, ClientConfig: libkibana.DefaultClientConfig()}
	kibanaCfg.Host = "testKibana:12345"
	client := kibana.NewConnectingClient(&kibanaCfg)
	cfg := &config.KibanaAgentConfig{Cache: &config.Cache{Expiration: 5 * time.Minute}}
	f := agentcfg.NewFetcher(client, cfg.Cache.Expiration)
	handler := NewHandler(f, cfg, "default")
	_, spans, _ := apmtest.WithTransaction(func(ctx context.Context) {
		// When the handler is called with a context containing
		// a transaction, the underlying Kibana query should create a span
		r := httptest.NewRequest(http.MethodPost, "/backend", convert.ToReader(m{
			"service": m{"name": "opbeans"}}))
		sendRequest(handler, r.WithContext(ctx))
	})
	require.Len(t, spans, 1)
	assert.Equal(t, "app", spans[0].Type)
}

func sendRequest(h request.Handler, r *http.Request) *httptest.ResponseRecorder {
	ctx, recorder := newRequestContext(r)
	h(ctx)
	return recorder
}

func newRequestContext(r *http.Request) (*request.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx := request.NewContext()
	ctx.Reset(w, r)
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

type recordingKibanaClient struct {
	kibana.Client
	requests []*http.Request
}

func (c *recordingKibanaClient) Send(ctx context.Context, method string, path string, params url.Values, header http.Header, body io.Reader) (*http.Response, error) {
	req := httptest.NewRequest(method, path, body)
	req.URL.RawQuery = params.Encode()
	for k, values := range header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	c.requests = append(c.requests, req.WithContext(ctx))
	return c.Client.Send(ctx, method, path, params, header, body)
}
