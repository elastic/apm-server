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
	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
)

// AuthConfig holds config information related to authorization against the APM Server
// It supports Bearer and API Key based authorization configuration
type AuthConfig struct {
	BearerToken  string        `config:"bearer.token"`
	APIKeyConfig *APIKeyConfig `config:"api_key"`
}

type APIKeyConfig struct {
	Enabled  bool                  `config:"enabled"`
	Cache    *LimitedCache         `config:"cache"`
	ESConfig *elasticsearch.Config `config:"elasticsearch"`
}

func (c *APIKeyConfig) IsEnabled() bool {
	return c != nil && c.Enabled
}

func (c *AuthConfig) setup(log *logp.Logger, ucfg *common.Config, secretToken string, outputESCfg *common.Config) error {
	if secretToken != "" {
		if c.BearerToken == "" {
			c.BearerToken = secretToken
		} else {
			log.Warn("Ignoring `apm-server.secret_token` as `apm-server.authorization.bearer.token` is configured.")
		}
	}
	if !c.APIKeyConfig.IsEnabled() || outputESCfg == nil {
		return nil
	}

	// return if ES has been explicityl configured
	if ucfg != nil {
		if auth, err := ucfg.Child("authorization", -1); err == nil {
			if api_key, err := auth.Child("api_key", -1); err == nil {
				if api_key.HasField("elasticsearch") {
					return nil
				}
			}
		}
	}

	// set ES from configured output
	if err := outputESCfg.Unpack(c.APIKeyConfig.ESConfig); err != nil {
		return err
	}
	return nil
}

func defaultAuthConfig() *AuthConfig {
	return &AuthConfig{APIKeyConfig: &APIKeyConfig{
		Enabled:  false,
		Cache:    &LimitedCache{Expiration: 5 * time.Minute, Size: 1000},
		ESConfig: &elasticsearch.Config{Hosts: []string{"localhost:9200"}, Protocol: "http", Timeout: 10 * time.Second},
	}}
}
