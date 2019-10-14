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

	"github.com/elastic/beats/libbeat/common"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
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

func TestMux_backendAuthMeans(t *testing.T) {
	t.Run("no auth config", func(t *testing.T) {
		means, err := backendAuthMeans(config.DefaultConfig("8.0.0"))
		assert.NoError(t, err)
		assert.Empty(t, means)
	})

	t.Run("no elasticsearch config", func(t *testing.T) {
		means, err := backendAuthMeans(&config.Config{AuthConfig: &config.AuthConfig{APIKeyConfig: &config.APIKeyConfig{Enabled: true}}})
		assert.Error(t, err)
		assert.Nil(t, means)
	})

	t.Run("valid auth config", func(t *testing.T) {
		cfg := common.MustNewConfigFrom(`{"authorization":{"bearer.token":"1234","api_key":{"enabled":true,"elasticsearch":{"hosts":["localhost:9200"]}}}}`)
		authorizationCfg := config.DefaultConfig("8.0.0")
		require.NoError(t, cfg.Unpack(authorizationCfg))
		means, err := backendAuthMeans(authorizationCfg)
		require.NoError(t, err)
		require.Equal(t, 2, len(means))
		assert.NotNil(t, means[headers.Bearer])
		assert.NotNil(t, means[headers.APIKey])
	})

}
