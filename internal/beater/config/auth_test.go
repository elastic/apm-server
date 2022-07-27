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

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
)

func TestAPIKeyAgentAuth_ESConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg            *config.C
		esCfg          *config.C
		expectedConfig APIKeyAgentAuth
	}{
		"default": {
			cfg:            nil,
			expectedConfig: defaultAPIKeyAgentAuth(),
		},
		"ES config missing": {
			cfg: config.MustNewConfigFrom(`{"enabled": true}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 100,
				ESConfig:    elasticsearch.DefaultConfig(),
				configured:  true,
			},
		},
		"ES configured": {
			cfg:   config.MustNewConfigFrom(`{"enabled": true, "elasticsearch.timeout":"7s"}`),
			esCfg: config.MustNewConfigFrom(`{"hosts":["186.0.0.168:9200"]}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 100,
				ESConfig: &elasticsearch.Config{
					Hosts:            elasticsearch.Hosts{"localhost:9200"},
					Protocol:         "http",
					Timeout:          7 * time.Second,
					MaxRetries:       3,
					CompressionLevel: 5,
					Backoff:          elasticsearch.DefaultBackoffConfig,
				},
				configured:   true,
				esConfigured: true,
			},
		},
		"disabled with ES from output": {
			cfg:            nil,
			esCfg:          config.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`),
			expectedConfig: defaultAPIKeyAgentAuth(),
		},
		"ES from output": {
			cfg:   config.MustNewConfigFrom(`{"enabled": true, "limit": 20}`),
			esCfg: config.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"],"username":"foo","password":"bar"}`),
			expectedConfig: APIKeyAgentAuth{
				Enabled:     true,
				LimitPerMin: 20,
				ESConfig: &elasticsearch.Config{
					Timeout:          5 * time.Second,
					Username:         "foo",
					Password:         "bar",
					Protocol:         "http",
					Hosts:            elasticsearch.Hosts{"192.0.0.168:9200"},
					MaxRetries:       3,
					CompressionLevel: 5,
					Backoff:          elasticsearch.DefaultBackoffConfig,
				},
				configured: true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := config.NewConfig()
			if tc.cfg != nil {
				input.SetChild("auth.api_key", -1, tc.cfg)
			}
			cfg, err := NewConfig(input, tc.esCfg)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedConfig, cfg.AgentAuth.APIKey)
		})
	}
}

func TestAnonymousAgentAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg            *config.C
		expectedConfig AnonymousAgentAuth
	}{
		"default": {
			cfg:            config.NewConfig(),
			expectedConfig: defaultAnonymousAgentAuth(),
		},
		"allow_service": {
			cfg: config.MustNewConfigFrom(`{"auth.anonymous.allow_service":["service-one"]}`),
			expectedConfig: AnonymousAgentAuth{
				AllowAgent:   []string{"rum-js", "js-base"},
				AllowService: []string{"service-one"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
				enabledSet: false,
			},
		},
		"rum_enabled_anon_inferred": {
			cfg: config.MustNewConfigFrom(`{"auth.secret_token": "abc","rum.enabled":true,"auth.anonymous.allow_service":["service-one"]}`),
			expectedConfig: AnonymousAgentAuth{
				Enabled:      true,
				AllowAgent:   []string{"rum-js", "js-base"},
				AllowService: []string{"service-one"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
				enabledSet: false,
			},
		},
		"rum_enabled_anon_disabled": {
			cfg: config.MustNewConfigFrom(
				`{"auth.secret_token":"abc","rum.enabled":true,"auth.anonymous.enabled":false,"auth.anonymous.allow_service":["service-one"]}`,
			),
			expectedConfig: AnonymousAgentAuth{
				Enabled:      false,
				AllowAgent:   []string{"rum-js", "js-base"},
				AllowService: []string{"service-one"},
				RateLimit: RateLimit{
					EventLimit: 300,
					IPLimit:    1000,
				},
				enabledSet: true,
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
		cfg      *config.C
		expected string
	}{
		"default": {
			cfg:      config.NewConfig(),
			expected: "",
		},
		"secret_token_auth": {
			cfg:      config.MustNewConfigFrom(`{"auth.secret_token":"token-one"}`),
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
