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

package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/elasticsearch"
)

func TestAPIKeyAuthorizer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
                  "username": "api_key_username",
                  "application": {
                    "apm": {
                      "-": {"config_agent:read": true, "event:write": true, "sourcemap:write": false}
                    }
                  }
                }`))
	}))
	defer srv.Close()

	esConfig := elasticsearch.DefaultConfig()
	esConfig.Hosts = elasticsearch.Hosts{srv.URL}
	apikeyAuthConfig := config.APIKeyAgentAuth{Enabled: true, LimitPerMin: 1, ESConfig: esConfig}
	authenticator, err := NewAuthenticator(config.AgentAuth{APIKey: apikeyAuthConfig})
	require.NoError(t, err)

	credentials := base64.StdEncoding.EncodeToString([]byte("valid_id:key_value"))
	_, authz, err := authenticator.Authenticate(context.Background(), headers.APIKey, credentials)
	require.NoError(t, err)

	err = authz.Authorize(context.Background(), ActionAgentConfig, Resource{})
	assert.NoError(t, err)

	err = authz.Authorize(context.Background(), ActionEventIngest, Resource{})
	assert.NoError(t, err)

	err = authz.Authorize(context.Background(), ActionSourcemapUpload, Resource{})
	assert.EqualError(t, err, `unauthorized: API Key not permitted action "sourcemap:write"`)
	assert.True(t, errors.Is(err, ErrUnauthorized))

	err = authz.Authorize(context.Background(), "unknown", Resource{})
	assert.EqualError(t, err, `unknown action "unknown"`)
}
