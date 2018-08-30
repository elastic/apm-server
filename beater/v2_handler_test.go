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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/transform"
)

func TestInvalidContentType(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", nil)
	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestEmptyRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", nil)
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	handler := (&v2BackendRoute).Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestRequestDecoderError(t *testing.T) {
	req := httptest.NewRequest("POST", "/v2/intake", bytes.NewBufferString(`asdasd`))
	req.Header.Add("Content-Type", "application/x-ndjson")

	w := httptest.NewRecorder()

	c := defaultConfig("7.0.0")
	expectedErr := errors.New("Faulty decoder")
	faultyDecoder := func(r *http.Request) (map[string]interface{}, error) {
		return nil, expectedErr
	}
	testRouteWithFaultyDecoder := v2Route{
		routeType{
			v2backendHandler,
			func(*Config, decoder.ReqDecoder) decoder.ReqDecoder { return faultyDecoder },
			func(*Config) transform.Config { return transform.Config{} },
		},
	}

	handler := testRouteWithFaultyDecoder.Handler(c, nil)

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}
