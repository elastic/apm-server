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
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/authorization"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/tests"
)

type m map[string]interface{}

var (
	mockVersion = *common.MustNewVersion("7.3.0")
	mockEtag    = "1c9588f5a4da71cdef992981a9c9735c"
	emptyEtag   = fmt.Sprintf("%x", md5.New().Sum([]byte{}))
	successBody = `{"sampling_rate":"0.5"}` + "\n"
	emptyBody   = `{}` + "\n"

	testcases = map[string]struct {
		kbClient      kibana.Client
		requestHeader map[string]string
		queryParams   map[string]string
		method        string

		respStatus                             int
		respBody, respBodyToken                string
		respEtagHeader, respCacheControlHeader string
	}{
		"NotModified": {
			kbClient: tests.MockKibana(http.StatusOK, m{
				"_id": "1",
				"_source": m{
					"settings": m{
						"sampling_rate": 0.5,
					},
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
			respEtagHeader:         `"` + emptyEtag + `"`,
			respBody:               emptyBody,
			respBodyToken:          emptyBody,
		},

		"SendToKibanaFailed": {
			kbClient:               tests.MockKibana(http.StatusBadGateway, m{}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               beatertest.ResultErrWrap(agentcfg.ErrMsgSendToKibanaFailed),
			respBodyToken:          beatertest.ResultErrWrap(fmt.Sprintf("%s: testerror", agentcfg.ErrMsgSendToKibanaFailed)),
		},

		"MultipleConfigs": {
			kbClient:               tests.MockKibana(http.StatusMultipleChoices, m{"s1": 1}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               beatertest.ResultErrWrap(agentcfg.ErrMsgMultipleChoices),
			respBodyToken:          beatertest.ResultErrWrap(fmt.Sprintf("%s: {\\\"s1\\\":1}", agentcfg.ErrMsgMultipleChoices)),
		},

		"NoConnection": {
			kbClient:               tests.MockKibana(http.StatusServiceUnavailable, m{}, mockVersion, false),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               beatertest.ResultErrWrap(msgNoKibanaConnection),
			respBodyToken:          beatertest.ResultErrWrap(msgNoKibanaConnection),
		},

		"InvalidVersion": {
			kbClient: tests.MockKibana(http.StatusServiceUnavailable, m{},
				*common.MustNewVersion("7.2.0"), true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               beatertest.ResultErrWrap(msgKibanaVersionNotCompatible),
			respBodyToken: beatertest.ResultErrWrap(fmt.Sprintf("%s: min version 7.3.0, configured version 7.2.0",
				msgKibanaVersionNotCompatible)),
		},

		"NoService": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodGet,
			respStatus:             http.StatusBadRequest,
			respBody:               beatertest.ResultErrWrap(msgInvalidQuery),
			respBodyToken:          beatertest.ResultErrWrap(`service.name is required`),
			respCacheControlHeader: "max-age=300, must-revalidate",
		},

		"MethodNotAllowed": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodPut,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusMethodNotAllowed,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               beatertest.ResultErrWrap(msgMethodUnsupported),
			respBodyToken:          beatertest.ResultErrWrap(fmt.Sprintf("%s: PUT", msgMethodUnsupported)),
		},
	}
)

func TestAgentConfigHandler(t *testing.T) {
	var cfg = config.AgentConfig{Cache: &config.Cache{Expiration: 4 * time.Second}}

	for name, tc := range testcases {

		runTest := func(t *testing.T, body string, tokenSet bool) {
			h := Handler(tc.kbClient, &cfg)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, target(tc.queryParams), nil)
			for k, v := range tc.requestHeader {
				r.Header.Set(k, v)
			}
			ctx := &request.Context{}
			ctx.Reset(w, r)
			if tokenSet {
				ctx.Authorization = authorization.NewBearer("abc", "abc")
			} else {
				ctx.Authorization = &authorization.Allow{}
			}
			h(ctx)

			require.Equal(t, tc.respStatus, w.Code)
			require.Equal(t, tc.respCacheControlHeader, w.Header().Get(headers.CacheControl))
			require.Equal(t, tc.respEtagHeader, w.Header().Get(headers.Etag))
			if body == "" {
				assert.Empty(t, w.Body)
			} else {
				b, err := ioutil.ReadAll(w.Body)
				require.NoError(t, err)
				assert.Equal(t, body, string(b))
			}
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
	cfg := config.AgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	h := Handler(nil, &cfg)

	w := httptest.NewRecorder()
	ctx := &request.Context{}
	ctx.Reset(w, httptest.NewRequest(http.MethodGet, target(map[string]string{"service.name": "opbeans-ruby"}), nil))
	h(ctx)

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

	var cfg = config.AgentConfig{Cache: &config.Cache{Expiration: time.Nanosecond}}
	h := Handler(kb, &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{
		"service": m{"name": "opbeans-node"}}))
	ctx := &request.Context{}
	ctx.Reset(w, r)
	h(ctx)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
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
