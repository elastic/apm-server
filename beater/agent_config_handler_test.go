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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elastic/beats/libbeat/kibana"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/tests"
)

var testcases = map[string]struct {
	kbClient      *kibana.Client
	requestHeader map[string]string
	queryParams   map[string]string
	method        string

	respStatus                             int
	respBody                               bool
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
		}),
		method:                 http.MethodGet,
		requestHeader:          map[string]string{headerIfNoneMatch: `"` + "1" + `"`},
		queryParams:            map[string]string{"service.name": "opbeans-node"},
		respStatus:             http.StatusNotModified,
		respCacheControlHeader: "max-age=4, must-revalidate",
		respEtagHeader:         "\"1\"",
	},

	"ModifiedWithoutEtag": {
		kbClient: tests.MockKibana(http.StatusOK, m{
			"_source": m{
				"settings": m{
					"sampling_rate": 0.5,
				},
			},
		}),
		method:                 http.MethodGet,
		queryParams:            map[string]string{"service.name": "opbeans-java"},
		respStatus:             http.StatusOK,
		respCacheControlHeader: "max-age=4, must-revalidate",
		respBody:               true,
	},

	"ModifiedWithEtag": {
		kbClient: tests.MockKibana(http.StatusOK, m{
			"_id": "1",
			"_source": m{
				"settings": m{
					"sampling_rate": 0.5,
				},
			},
		}),
		method:                 http.MethodGet,
		requestHeader:          map[string]string{headerIfNoneMatch: "2"},
		queryParams:            map[string]string{"service.name": "opbeans-java"},
		respStatus:             http.StatusOK,
		respEtagHeader:         "\"1\"",
		respCacheControlHeader: "max-age=4, must-revalidate",
		respBody:               true,
	},

	"InternalError": {
		kbClient: tests.MockKibana(http.StatusExpectationFailed, m{
			"_id": "1", "_source": ""},
		),
		method:                 http.MethodGet,
		queryParams:            map[string]string{"service.name": "opbeans-ruby"},
		respStatus:             http.StatusServiceUnavailable,
		respCacheControlHeader: "max-age=300, must-revalidate",
		respBody:               true,
	},

	"StatusNotFoundError": {
		kbClient:               tests.MockKibana(http.StatusNotFound, m{}),
		method:                 http.MethodGet,
		queryParams:            map[string]string{"service.name": "opbeans-python"},
		respStatus:             http.StatusNotFound,
		respCacheControlHeader: "max-age=300, must-revalidate",
	},

	"NoService": {
		kbClient:               tests.MockKibana(http.StatusOK, m{}),
		method:                 http.MethodGet,
		respStatus:             http.StatusBadRequest,
		respBody:               true,
		respCacheControlHeader: "max-age=300, must-revalidate",
	},

	"MethodNotAllowed": {
		kbClient:               tests.MockKibana(http.StatusOK, m{}),
		method:                 http.MethodPut,
		respStatus:             http.StatusMethodNotAllowed,
		respCacheControlHeader: "max-age=300, must-revalidate",
	},
}

func TestAgentConfigHandler(t *testing.T) {
	var cfg = agentConfig{Cache: &Cache{Expiration: 4 * time.Second}}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			h := agentConfigHandler(tc.kbClient, true, &cfg, "")
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, target(tc.queryParams), nil)
			for k, v := range tc.requestHeader {
				r.Header.Set(k, v)
			}
			h.ServeHTTP(w, r)

			assert.Equal(t, tc.respStatus, w.Code)
			assert.Equal(t, tc.respCacheControlHeader, w.Header().Get(headerCacheControl))
			assert.Equal(t, tc.respEtagHeader, w.Header().Get(headerEtag))
			if tc.respBody {
				assert.NotEmpty(t, w.Body)
			} else {
				assert.Empty(t, w.Body)
			}
		})
	}
}

func TestAgentConfigDisabled(t *testing.T) {
	cfg := agentConfig{Cache: &Cache{Expiration: time.Nanosecond}}
	h := agentConfigHandler(nil, false, &cfg, "")

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
	})

	var cfg = agentConfig{Cache: &Cache{Expiration: time.Nanosecond}}
	h := agentConfigHandler(kb, true, &cfg, "")

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
