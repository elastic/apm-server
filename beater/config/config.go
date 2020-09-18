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
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/kibana"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const (
	// DefaultPort of APM Server
	DefaultPort = "8200"

	msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"
)

type KibanaConfig struct {
	Enabled             bool `config:"enabled"`
	kibana.ClientConfig `config:",inline"`
}

func (k *KibanaConfig) Unpack(cfg *common.Config) error {
	if err := cfg.Unpack(&k.ClientConfig); err != nil {
		return err
	}
	k.Enabled = cfg.Enabled()
	k.Host = strings.TrimRight(k.Host, "/")

	return nil
}

func defaultKibanaConfig() KibanaConfig {
	return KibanaConfig{
		Enabled:      false,
		ClientConfig: kibana.DefaultClientConfig(),
	}
}

// Config holds configuration information nested under the key `apm-server`
type Config struct {
	Host                string                  `config:"host"`
	MaxHeaderSize       int                     `config:"max_header_size"`
	IdleTimeout         time.Duration           `config:"idle_timeout"`
	ReadTimeout         time.Duration           `config:"read_timeout"`
	WriteTimeout        time.Duration           `config:"write_timeout"`
	MaxEventSize        int                     `config:"max_event_size"`
	ShutdownTimeout     time.Duration           `config:"shutdown_timeout"`
	TLS                 *tlscommon.ServerConfig `config:"ssl"`
	MaxConnections      int                     `config:"max_connections"`
	Expvar              *ExpvarConfig           `config:"expvar"`
	AugmentEnabled      bool                    `config:"capture_personal_data"`
	SelfInstrumentation *InstrumentationConfig  `config:"instrumentation"`
	RumConfig           *RumConfig              `config:"rum"`
	Register            *RegisterConfig         `config:"register"`
	Mode                Mode                    `config:"mode"`
	Kibana              KibanaConfig            `config:"kibana"`
	AgentConfig         *AgentConfig            `config:"agent.config"`
	SecretToken         string                  `config:"secret_token"`
	APIKeyConfig        *APIKeyConfig           `config:"api_key"`
	JaegerConfig        JaegerConfig            `config:"jaeger"`
	Aggregation         AggregationConfig       `config:"aggregation"`
	Sampling            SamplingConfig          `config:"sampling"`

	Pipeline string
}

// ExpvarConfig holds config information about exposing expvar
type ExpvarConfig struct {
	Enabled *bool  `config:"enabled"`
	URL     string `config:"url"`
}

// AgentConfig holds remote agent config information
type AgentConfig struct {
	Cache *Cache `config:"cache"`
}

// Cache holds config information about cache expiration
type Cache struct {
	Expiration time.Duration `config:"expiration"`
}

// NewConfig creates a Config struct based on the default config and the given input params
func NewConfig(ucfg *common.Config, outputESCfg *common.Config) (*Config, error) {
	logger := logp.NewLogger(logs.Config)
	c := DefaultConfig()
	if err := ucfg.Unpack(c); err != nil {
		return nil, errors.Wrap(err, "Error processing configuration")
	}

	if float64(int(c.AgentConfig.Cache.Expiration.Seconds())) != c.AgentConfig.Cache.Expiration.Seconds() {
		return nil, errors.New(msgInvalidConfigAgentCfg)
	}

	if outputESCfg != nil && (outputESCfg.HasField("pipeline") || outputESCfg.HasField("pipelines")) {
		c.Pipeline = ""
	}

	if err := c.RumConfig.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.APIKeyConfig.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.SelfInstrumentation.setup(logger); err != nil {
		return nil, err
	}

	if err := c.JaegerConfig.setup(c); err != nil {
		return nil, err
	}

	if c.Sampling.Tail != nil {
		if err := c.Sampling.Tail.setup(logger, outputESCfg); err != nil {
			return nil, err
		}
	}

	if !c.Sampling.KeepUnsampled && !c.Aggregation.Transactions.Enabled {
		// Unsampled transactions should only be dropped
		// when transaction aggregation is enabled in the
		// server. This means the aggregations performed
		// by the APM UI will not have access to a complete
		// representation of the latency distribution.
		logger.Warn("" +
			"apm-server.sampling.keep_unsampled and " +
			"apm-server.aggregation.transactions.enabled are both false, " +
			"which will lead to incorrect metrics being reported in the APM UI",
		)
	}
	return c, nil
}

// IsEnabled indicates whether expvar is enabled or not
func (c *ExpvarConfig) IsEnabled() bool {
	return c != nil && (c.Enabled == nil || *c.Enabled)
}

// DefaultConfig returns a config with default settings for `apm-server` config options.
func DefaultConfig() *Config {
	return &Config{
		Host:            net.JoinHostPort("localhost", DefaultPort),
		MaxHeaderSize:   1 * 1024 * 1024, // 1mb
		MaxConnections:  0,               // unlimited
		IdleTimeout:     45 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		MaxEventSize:    300 * 1024, // 300 kb
		ShutdownTimeout: 5 * time.Second,
		AugmentEnabled:  true,
		Expvar: &ExpvarConfig{
			Enabled: new(bool),
			URL:     "/debug/vars",
		},
		RumConfig:    defaultRum(),
		Register:     defaultRegisterConfig(true),
		Mode:         ModeProduction,
		Kibana:       defaultKibanaConfig(),
		AgentConfig:  &AgentConfig{Cache: &Cache{Expiration: 30 * time.Second}},
		Pipeline:     defaultAPMPipeline,
		APIKeyConfig: defaultAPIKeyConfig(),
		JaegerConfig: defaultJaeger(),
		Aggregation:  defaultAggregationConfig(),
		Sampling:     defaultSamplingConfig(),
	}
}
