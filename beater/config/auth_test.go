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

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestAPIKeyAgentAuth_ESConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg            *common.Config
		esCfg          *common.Config
		expectedConfig APIKeyAgentAuth
	}{
		"default": {
			cfg:            nil,
			expectedConfig: defaultAPIKeyAgentAuth(),
		},
		"ES config missing": {
			cfg: common.MustNewConfigFrom(`{"enabled": true}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 100,
				ESConfig:    elasticsearch.DefaultConfig(),
				configured:  true,
			},
		},
		"ES configured": {
			cfg:   common.MustNewConfigFrom(`{"enabled": true, "elasticsearch.timeout":"7s"}`),
			esCfg: common.MustNewConfigFrom(`{"hosts":["186.0.0.168:9200"]}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 100,
				ESConfig: &elasticsearch.Config{
					Hosts:      elasticsearch.Hosts{"localhost:9200"},
					Protocol:   "http",
					Timeout:    7 * time.Second,
					MaxRetries: 3,
					Backoff:    elasticsearch.DefaultBackoffConfig,
				},
				configured:   true,
				esConfigured: true,
			},
		},
		"disabled with ES from output": {
			cfg:            nil,
			esCfg:          common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`),
			expectedConfig: defaultAPIKeyAgentAuth(),
		},
		"ES from output": {
			cfg:   common.MustNewConfigFrom(`{"enabled": true, "limit": 20}`),
			esCfg: common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"],"username":"foo","password":"bar"}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 20,
				ESConfig: &elasticsearch.Config{
					Timeout:    5 * time.Second,
					Username:   "foo",
					Password:   "bar",
					Protocol:   "http",
					Hosts:      elasticsearch.Hosts{"192.0.0.168:9200"},
					MaxRetries: 3,
					Backoff:    elasticsearch.DefaultBackoffConfig,
				},
				configured: true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			for _, key := range []string{"api_key", "auth.api_key"} {
				input := common.NewConfig()
				if tc.cfg != nil {
					input.SetChild(key, -1, tc.cfg)
				}
				cfg, err := NewConfig(input, tc.esCfg)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedConfig, cfg.AgentAuth.APIKey)
			}
		})
	}
}

func TestAnonymousAgentAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg            *common.Config
		expectedConfig AnonymousAgentAuth
	}{
		"default": {
			cfg:            common.NewConfig(),
			expectedConfig: defaultAnonymousAgentAuth(),
		},
		"allow_service": {
			cfg: common.MustNewConfigFrom(`{"auth.anonymous.allow_service":["service-one"]}`),
			expectedConfig: AnonymousAgentAuth{
				AllowAgent:   []string{"rum-js"},
				AllowService: []string{"service-one"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
				configured: true,
			},
		},
		"deprecated_rum_allow_service_names": {
			cfg: common.MustNewConfigFrom(`{"rum.allow_service_names":["service-two"]}`),
			expectedConfig: AnonymousAgentAuth{
				AllowAgent:   []string{"rum-js"},
				AllowService: []string{"service-two"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
			},
		},
		"deprecated_rum_allow_service_names_conflict": {
			cfg: common.MustNewConfigFrom(`{"auth.anonymous.allow_service":["service-one"], "rum.allow_service_names":["service-two"]}`),
			expectedConfig: AnonymousAgentAuth{
				AllowAgent:   []string{"rum-js"},
				AllowService: []string{"service-one"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
				configured: true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := NewConfig(tc.cfg, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedConfig, cfg.AgentAuth.Anonymous)
		})
	}
}

func TestSecretTokenAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg      *common.Config
		expected string
	}{
		"default": {
			cfg:      common.NewConfig(),
			expected: "",
		},
		"secret_token_auth": {
			cfg:      common.MustNewConfigFrom(`{"auth.secret_token":"token-one"}`),
			expected: "token-one",
		},
		"deprecated_secret_token": {
			cfg:      common.MustNewConfigFrom(`{"secret_token":"token-two"}`),
			expected: "token-two",
		},
		"deprecated_secret_token_conflict": {
			cfg:      common.MustNewConfigFrom(`{"auth.secret_token":"token-one","secret_token":"token-two"}`),
			expected: "token-one",
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := NewConfig(tc.cfg, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.AgentAuth.SecretToken)
		})
	}
}
