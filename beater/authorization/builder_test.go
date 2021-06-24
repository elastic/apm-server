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

package authorization

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
)

func TestBuilder(t *testing.T) {
	for name, tc := range map[string]struct {
		withBearer    bool
		withApikey    bool
		withAnonymous bool
		bearer        *bearerBuilder
	}{
		"no auth":   {},
		"bearer":    {withBearer: true, bearer: &bearerBuilder{"xvz"}},
		"apikey":    {withApikey: true},
		"anonymous": {withAnonymous: true},
		"all": {
			withApikey:    true,
			withBearer:    true,
			withAnonymous: true,
			bearer:        &bearerBuilder{"xvz"},
		},
	} {

		setup := func() *Builder {
			var cfg config.AgentAuth
			if tc.withBearer {
				cfg.SecretToken = "xvz"
			}
			if tc.withApikey {
				cfg.APIKey = config.APIKeyAgentAuth{
					Enabled: true, LimitPerMin: 100,
					ESConfig: elasticsearch.DefaultConfig(),
				}
			}
			if tc.withAnonymous {
				cfg.Anonymous.Enabled = true
			}
			builder, err := NewBuilder(cfg)
			require.NoError(t, err)
			return builder
		}
		t.Run("NewBuilder"+name, func(t *testing.T) {
			builder := setup()
			assert.Equal(t, tc.bearer, builder.bearer)
			if tc.withApikey {
				assert.NotNil(t, builder.apikey)
				assert.Equal(t, 100, builder.apikey.cache.size)
				assert.NotNil(t, builder.apikey.esClient)
			}
		})
		t.Run("ForPrivilege"+name, func(t *testing.T) {
			builder := setup()
			h := builder.ForPrivilege(PrivilegeSourcemapWrite.Action)
			assert.Equal(t, builder.bearer, h.bearer)
			if tc.withApikey {
				assert.Equal(t, []elasticsearch.PrivilegeAction{}, builder.apikey.anyOfPrivileges)
				assert.Equal(t, []elasticsearch.PrivilegeAction{PrivilegeSourcemapWrite.Action}, h.apikey.anyOfPrivileges)
				assert.Equal(t, builder.apikey.esClient, h.apikey.esClient)
				assert.Equal(t, builder.apikey.cache, h.apikey.cache)
			}
		})
		t.Run("AuthorizationFor"+name, func(t *testing.T) {
			builder := setup()
			h := builder.ForPrivilege(PrivilegeSourcemapWrite.Action)
			auth := h.AuthorizationFor("ApiKey", "")
			if tc.withApikey {
				assert.IsType(t, &apikeyAuth{}, auth)
			} else if tc.withBearer || tc.withAnonymous {
				assert.Equal(t, denyAuth{}, auth)
			} else {
				assert.Equal(t, allowAuth{}, auth)
			}

			auth = h.AuthorizationFor("Bearer", "")
			if tc.withBearer {
				assert.IsType(t, &bearerAuth{}, auth)
			} else if tc.withApikey || tc.withAnonymous {
				assert.Equal(t, denyAuth{}, auth)
			} else {
				assert.Equal(t, allowAuth{}, auth)
			}

			auth = h.AuthorizationFor("Anything", "")
			if tc.withBearer || tc.withApikey || tc.withAnonymous {
				assert.Equal(t, denyAuth{reason: `unknown Authorization kind Anything: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'`}, auth)
			} else {
				assert.Equal(t, allowAuth{}, auth)
			}

			auth = h.AuthorizationFor("", "Value")
			if tc.withAnonymous {
				assert.Equal(t, &anonymousAuth{
					allowedAgents:   make(map[string]bool),
					allowedServices: make(map[string]bool),
				}, auth)
			} else if tc.withBearer || tc.withApikey {
				assert.Equal(t, denyAuth{reason: `missing or improperly formatted Authorization header: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'`}, auth)
			} else {
				assert.Equal(t, allowAuth{}, auth)
			}
		})
	}

}

func TestHandlerWithAnonymousDisabled(t *testing.T) {
	builder, _ := NewBuilder(config.AgentAuth{
		SecretToken: "abc123",
		Anonymous: config.AnonymousAgentAuth{
			Enabled: true,
		},
	})
	anonymousEnabled := builder.ForAnyOfPrivileges()
	anonymousDisabled := anonymousEnabled.WithAnonymousDisabled()

	result, err := anonymousDisabled.AuthorizationFor("", "").AuthorizedFor(context.Background(), Resource{})
	require.NoError(t, err)
	assert.Equal(t, Result{Authorized: false, Reason: "missing or improperly formatted Authorization header: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'"}, result)

	result, err = anonymousEnabled.AuthorizationFor("", "").AuthorizedFor(context.Background(), Resource{})
	require.NoError(t, err)
	assert.Equal(t, Result{Authorized: true, Anonymous: true}, result)

}
