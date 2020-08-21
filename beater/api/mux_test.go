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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/publish"
)

func requestToMuxerWithPattern(cfg *config.Config, pattern string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(http.MethodPost, pattern, nil)
	return requestToMuxer(cfg, r)
}
func requestToMuxerWithHeader(cfg *config.Config, pattern string, method string, header map[string]string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(method, pattern, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return requestToMuxer(cfg, r)
}

func requestToMuxer(cfg *config.Config, r *http.Request) (*httptest.ResponseRecorder, error) {
	mux, err := NewMux(cfg, beatertest.NilReporter)
	if err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	h, _ := mux.Handler(r)
	h.ServeHTTP(w, r)
	return w, nil
}

func testHandler(t *testing.T, fn func(*config.Config, *authorization.Builder, publish.Reporter) (request.Handler, error)) request.Handler {
	cfg := config.DefaultConfig()
	builder, err := authorization.NewBuilder(cfg)
	require.NoError(t, err)
	h, err := fn(cfg, builder, beatertest.NilReporter)
	require.NoError(t, err)
	return h
}
