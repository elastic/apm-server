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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/kibana"
	"github.com/elastic/apm-server/tests"
)

var (
	mockVersion = *common.MustNewVersion("7.3.0")
	mockEtag    = "1c9588f5a4da71cdef992981a9c9735c"
	errWrap     = func(s string) string { return fmt.Sprintf("{\"error\":\"%+v\"}\n", s) }
	successBody = `{"sampling_rate":"0.5"}` + "\n"

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

		"SendToKibanaFailed": {
			kbClient:               tests.MockKibana(http.StatusBadGateway, m{}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(agentcfg.ErrMsgSendToKibanaFailed),
			respBodyToken:          errWrap(fmt.Sprintf("%s: testerror", agentcfg.ErrMsgSendToKibanaFailed)),
		},

		"MultipleConfigs": {
			kbClient:               tests.MockKibana(http.StatusMultipleChoices, m{"s1": 1}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-ruby"},
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(agentcfg.ErrMsgMultipleChoices),
			respBodyToken:          errWrap(fmt.Sprintf("%s: {\\\"s1\\\":1}", agentcfg.ErrMsgMultipleChoices)),
		},

		"NoConnection": {
			kbClient:               tests.MockKibana(http.StatusServiceUnavailable, m{}, mockVersion, false),
			method:                 http.MethodGet,
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(errMsgNoKibanaConnection),
			respBodyToken:          errWrap(errMsgNoKibanaConnection),
		},

		"InvalidVersion": {
			kbClient:               tests.MockKibana(http.StatusServiceUnavailable, m{}, *common.MustNewVersion("7.2.0"), true),
			method:                 http.MethodGet,
			respStatus:             http.StatusServiceUnavailable,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(errMsgKibanaVersionNotCompatible),
			respBodyToken: errWrap("min required Kibana version 7.3.0," +
				" configured Kibana version {version:7.2.0 Major:7 Minor:2 Bugfix:0 Meta:}"),
		},

		"StatusNotFoundError": {
			kbClient:               tests.MockKibana(http.StatusNotFound, m{}, mockVersion, true),
			method:                 http.MethodGet,
			queryParams:            map[string]string{"service.name": "opbeans-python"},
			respStatus:             http.StatusNotFound,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(errMsgConfigNotFound),
			respBodyToken:          errWrap(fmt.Sprintf("%s for opbeans-python", errMsgConfigNotFound)),
		},

		"NoService": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodGet,
			respStatus:             http.StatusBadRequest,
			respBody:               errWrap(errMsgInvalidQuery),
			respBodyToken:          errWrap(`service.name is required`),
			respCacheControlHeader: "max-age=300, must-revalidate",
		},

		"MethodNotAllowed": {
			kbClient:               tests.MockKibana(http.StatusOK, m{}, mockVersion, true),
			method:                 http.MethodPut,
			respStatus:             http.StatusMethodNotAllowed,
			respCacheControlHeader: "max-age=300, must-revalidate",
			respBody:               errWrap(errMsgMethodUnsupported),
			respBodyToken:          errWrap(fmt.Sprintf("%s: PUT", errMsgMethodUnsupported)),
		},
	}
)

func TestAgentConfigHandler(t *testing.T) {
	var cfg = agentConfig{Cache: &Cache{Expiration: 4 * time.Second}}

	for name, tc := range testcases {

		runTest := func(t *testing.T, body, token string) {
			h := agentConfigHandler(tc.kbClient, &cfg, token)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, target(tc.queryParams), nil)
			for k, v := range tc.requestHeader {
				r.Header.Set(k, v)
			}
			r.Header.Set("Authorization", "Bearer "+token)
			h.ServeHTTP(w, r)

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
			runTest(t, tc.respBody, "")
		})

		t.Run(name+"WithSecretToken", func(t *testing.T) {
			runTest(t, tc.respBodyToken, "1234")
		})
	}
}

func TestAgentConfigDisabled(t *testing.T) {
	cfg := agentConfig{Cache: &Cache{Expiration: time.Nanosecond}}
	h := agentConfigHandler(nil, &cfg, "")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestAgentConfigHandlerPostOk(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": 0.5,
			},
		},
	}, mockVersion, true)

	var cfg = agentConfig{Cache: &Cache{Expiration: time.Nanosecond}}
	h := agentConfigHandler(kb, &cfg, "")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{
		"service": m{"name": "opbeans-node"}}))
	h.ServeHTTP(w, r)

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
