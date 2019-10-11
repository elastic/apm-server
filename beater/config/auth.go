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
	"time"

	"github.com/elastic/beats/libbeat/common"
)

// AuthConfig holds config information related to authorization against the APM Server
// It supports Bearer and API Key based authorization configuration
type AuthConfig struct {
	BearerToken  string        `config:"bearer.token"`
	APIKeyConfig *APIKeyConfig `config:"api_key"`
}

type APIKeyConfig struct {
	Enabled  bool            `config:"enabled"`
	Cache    *LimitedCache   `config:"cache"`
	ESConfig *APIKeyESConfig `config:"elasticsearch"`
}

type APIKeyESConfig common.Config

func (c *APIKeyConfig) IsEnabled() bool {
	return c != nil && c.Enabled
}

func (c *Config) HasAPIKeyESConfigured() bool {
	return c.AuthConfig != nil && c.AuthConfig.APIKeyConfig != nil && c.AuthConfig.APIKeyConfig.ESConfig == nil
}

// Unpack Elasticsearch configuration for API Key authorization
func (c *APIKeyESConfig) Unpack(inp *common.Config) error {
	cfg := common.MustNewConfigFrom(map[string]interface{}{
		"hosts":    []string{"localhost:9200"},
		"protocol": "http",
		"timeout":  "10s",
	})
	if err := inp.Unpack(&cfg); err != nil {
		return err
	}
	*c = APIKeyESConfig(*cfg)
	return nil
}

func (c *APIKeyESConfig) CommonConfig() *common.Config {
	if c == nil {
		return nil
	}
	esCfg := common.Config(*c)
	return &esCfg
}

func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		APIKeyConfig: &APIKeyConfig{
			Enabled: false,
			Cache:   &LimitedCache{Expiration: 5 * time.Minute, Size: 1000},
		},
	}
}
