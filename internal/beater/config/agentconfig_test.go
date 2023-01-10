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

	"github.com/elastic/elastic-agent-libs/config"
)

// TestAgentConfig tests server configuration the Elasticsearch-based or legacy Kibana-based agent config implementation.
func TestAgentConfig(t *testing.T) {
	t.Run("InvalidValueTooSmall", func(t *testing.T) {
		cfg, err := NewConfig(config.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("InvalidUnit", func(t *testing.T) {
		cfg, err := NewConfig(config.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "1230ms"}), nil)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Valid", func(t *testing.T) {
		cfg, err := NewConfig(config.MustNewConfigFrom(map[string]string{"agent.config.cache.expiration": "123000ms"}), nil)
		require.NoError(t, err)
		assert.Equal(t, time.Second*123, cfg.AgentConfig.Cache.Expiration)
	})
}

func TestAgentConfigs(t *testing.T) {
	cfg, err := NewConfig(config.MustNewConfigFrom(`{"agent_config":[{"service.environment":"production","config":{"transaction_sample_rate":0.5}}]}`), nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.FleetAgentConfigs, 1)
	assert.NotEmpty(t, cfg.FleetAgentConfigs[0].Etag)

	// The "config" attribute may come through as `null` when no config attributes are defined.
	cfg, err = NewConfig(config.MustNewConfigFrom(`{"agent_config":[{"service.environment":"production","config":null}]}`), nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.FleetAgentConfigs, 1)
	assert.NotEmpty(t, cfg.FleetAgentConfigs[0].Etag)
}
