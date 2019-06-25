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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/convert"
	"github.com/elastic/apm-server/tests"
)

var cfg = agentcfg.Config{CacheExpiration: time.Nanosecond}

func TestAgentConfigHandlerGetOk(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": 0.5,
			},
		},
	})

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config?service.name=opbeans-python", nil)
	h.ServeHTTP(w, r)

	etagHeader := w.Header().Get("Etag")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "1", etagHeader, etagHeader)
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

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/config", convert.ToReader(m{
		"service": m{"name": "opbeans-node"}}))
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestAgentConfigHandlerBadMethod(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{})

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/config?service.name=opbeans-java", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, w.Body.String())
}

func TestAgentConfigHandlerNoService(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{})

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestAgentConfigHandlerNotFound(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{})

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config?service.name=opbeans-go", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestAgentConfigHandlerInternalError(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1", "_source": ""},
	)

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config?service.name=opbeans-ruby", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

func TestAgentConfigHandlerNotModified(t *testing.T) {

	kb := tests.MockKibana(http.StatusOK, m{
		"_id": "1",
		"_source": m{
			"settings": m{
				"sampling_rate": 0.5,
			},
		},
	})

	h := agentConfigHandler(kb, "", &cfg)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config?service.name=opbeans-js", nil)
	r.Header.Set("If-None-Match", "1")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotModified, w.Code, w.Body.String())
}
