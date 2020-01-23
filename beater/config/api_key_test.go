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

	"github.com/elastic/beats/libbeat/logp"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/elasticsearch"
)

func TestAPIKeyConfig_IsEnabled(t *testing.T) {
	assert.False(t, (&APIKeyConfig{}).IsEnabled())
	assert.False(t, defaultAPIKeyConfig().IsEnabled())
	assert.True(t, (&APIKeyConfig{Enabled: true}).IsEnabled())
}

func TestAPIKeyConfig_ESConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg   *APIKeyConfig
		esCfg *common.Config

		expectedConfig *APIKeyConfig
		expectedErr    error
	}{
		"default": {
			cfg:            defaultAPIKeyConfig(),
			expectedConfig: defaultAPIKeyConfig(),
		},
		"ES config missing": {
			cfg: &APIKeyConfig{Enabled: true, LimitPerMin: apiKeyLimit},
			expectedConfig: &APIKeyConfig{
				Enabled:     true,
				LimitPerMin: apiKeyLimit,
				ESConfig:    elasticsearch.DefaultConfig()},
		},
		"ES configured": {
			cfg: &APIKeyConfig{
				Enabled:  true,
				ESConfig: &elasticsearch.Config{Hosts: elasticsearch.Hosts{"192.0.0.1:9200"}}},
			esCfg: common.MustNewConfigFrom(`{"hosts":["186.0.0.168:9200"]}`),
			expectedConfig: &APIKeyConfig{
				Enabled:  true,
				ESConfig: &elasticsearch.Config{Hosts: elasticsearch.Hosts{"192.0.0.1:9200"}}},
		},
		"disabled with ES from output": {
			cfg:            defaultAPIKeyConfig(),
			esCfg:          common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"]}`),
			expectedConfig: defaultAPIKeyConfig(),
		},
		"ES from output": {
			cfg:   &APIKeyConfig{Enabled: true, LimitPerMin: 20},
			esCfg: common.MustNewConfigFrom(`{"hosts":["192.0.0.168:9200"],"username":"foo","password":"bar"}`),
			expectedConfig: &APIKeyConfig{
				Enabled:     true,
				LimitPerMin: 20,
				ESConfig: &elasticsearch.Config{
					Timeout:  5 * time.Second,
					Username: "foo",
					Password: "bar",
					Protocol: "http",
					Hosts:    elasticsearch.Hosts{"192.0.0.168:9200"}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.setup(logp.NewLogger("api_key"), tc.esCfg)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, tc.expectedConfig, tc.cfg)

		})
	}

}
