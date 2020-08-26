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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
)

func TestBuilder(t *testing.T) {
	for name, tc := range map[string]struct {
		withBearer, withApikey bool
		bearer                 *bearerBuilder
		fallback               Authorization
	}{
		"no auth": {fallback: AllowAuth{}},
		"bearer":  {withBearer: true, fallback: DenyAuth{}, bearer: &bearerBuilder{"xvz"}},
		"apikey":  {withApikey: true, fallback: DenyAuth{}},
		"all":     {withApikey: true, withBearer: true, fallback: DenyAuth{}, bearer: &bearerBuilder{"xvz"}},
	} {

		setup := func() *Builder {
			cfg := config.DefaultConfig()

			if tc.withBearer {
				cfg.SecretToken = "xvz"
			}
			if tc.withApikey {
				cfg.APIKeyConfig = &config.APIKeyConfig{
					Enabled: true, LimitPerMin: 100, ESConfig: elasticsearch.DefaultConfig()}
			}

			builder, err := NewBuilder(cfg)
			require.NoError(t, err)
			return builder
		}
		t.Run("NewBuilder"+name, func(t *testing.T) {
			builder := setup()
			assert.Equal(t, tc.fallback, builder.fallback)
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
			assert.Equal(t, builder.fallback, h.fallback)
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
			} else {
				assert.Equal(t, h.fallback, auth)
			}

			auth = h.AuthorizationFor("Bearer", "")
			if tc.withBearer {
				assert.IsType(t, &bearerAuth{}, auth)
			} else {
				assert.Equal(t, h.fallback, auth)
			}

			auth = h.AuthorizationFor("Anything", "")
			assert.Equal(t, h.fallback, auth)
		})
	}

}
