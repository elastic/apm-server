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
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/transport/tlscommon"

	"github.com/elastic/apm-server/internal/logs"
)

const (
	// DefaultPort of APM Server
	DefaultPort = "8200"

	msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"
)

// Config holds configuration information nested under the key `apm-server`
type Config struct {
	// Host holds the hostname or address that the server should bind to
	// when listening for requests from agents.
	Host string `config:"host"`

	// AgentAuth holds agent auth config.
	AgentAuth AgentAuth `config:"auth"`

	MaxHeaderSize             int                     `config:"max_header_size"`
	IdleTimeout               time.Duration           `config:"idle_timeout"`
	ReadTimeout               time.Duration           `config:"read_timeout"`
	WriteTimeout              time.Duration           `config:"write_timeout"`
	MaxEventSize              int                     `config:"max_event_size"`
	ShutdownTimeout           time.Duration           `config:"shutdown_timeout"`
	TLS                       *tlscommon.ServerConfig `config:"ssl"`
	MaxConnections            int                     `config:"max_connections"`
	ResponseHeaders           map[string][]string     `config:"response_headers"`
	Expvar                    ExpvarConfig            `config:"expvar"`
	Pprof                     PprofConfig             `config:"pprof"`
	AugmentEnabled            bool                    `config:"capture_personal_data"`
	RumConfig                 RumConfig               `config:"rum"`
	Kibana                    KibanaConfig            `config:"kibana"`
	KibanaAgentConfig         KibanaAgentConfig       `config:"agent.config"`
	Aggregation               AggregationConfig       `config:"aggregation"`
	Sampling                  SamplingConfig          `config:"sampling"`
	Profiling                 ProfilingConfig         `config:"profiling"`
	DataStreams               DataStreamsConfig       `config:"data_streams"`
	DefaultServiceEnvironment string                  `config:"default_service_environment"`
	JavaAttacherConfig        JavaAttacherConfig      `config:"java_attacher"`

	AgentConfigs []AgentConfig `config:"agent_config"`

	// WaitReadyInterval holds the interval for checks when waiting for
	// the integration package to be installed, and for checking the
	// Elasticsearch license level.
	WaitReadyInterval time.Duration `config:"wait_ready_interval"`

	// MaxConcurrentDecoders sets the limit on the number of concurrent batches
	// that can be decoded at one time. This effectively limits the amount of
	// memory consumed by the processors decodeing the incoming intake events.
	// This setting is beta and subject to breaking changes and removal.
	// If set to zero, it will automatically tuned to provide reasonable
	// performance based on the memory memory limit.
	MaxConcurrentDecoders uint `config:"max_concurrent_decoders"`
}

// NewConfig creates a Config struct based on the default config and the given input params
func NewConfig(ucfg *config.C, outputESCfg *config.C) (*Config, error) {
	logger := logp.NewLogger(logs.Config)
	c := DefaultConfig()
	if err := ucfg.Unpack(c); err != nil {
		return nil, errors.Wrap(err, "Error processing configuration")
	}

	if float64(int(c.KibanaAgentConfig.Cache.Expiration.Seconds())) != c.KibanaAgentConfig.Cache.Expiration.Seconds() {
		return nil, errors.New(msgInvalidConfigAgentCfg)
	}

	for i := range c.AgentConfigs {
		if err := c.AgentConfigs[i].setup(); err != nil {
			return nil, err
		}
	}

	if err := c.RumConfig.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.AgentAuth.setAnonymousDefaults(logger, c.RumConfig.Enabled); err != nil {
		return nil, err
	}

	if err := c.AgentAuth.APIKey.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.Sampling.Tail.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.JavaAttacherConfig.setup(); err != nil {
		logger.Warnf("failed to setup java-attacher: %v", err)
		c.JavaAttacherConfig = defaultJavaAttacherConfig()
	}

	if err := c.Profiling.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	return c, nil
}

// DefaultConfig returns a config with default settings for `apm-server` config options.
func DefaultConfig() *Config {
	return &Config{
		Host:            net.JoinHostPort("127.0.0.1", DefaultPort),
		MaxHeaderSize:   1 * 1024 * 1024, // 1mb
		MaxConnections:  0,               // unlimited
		IdleTimeout:     45 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		MaxEventSize:    300 * 1024, // 300 kb
		ShutdownTimeout: 30 * time.Second,
		AugmentEnabled:  true,
		Expvar: ExpvarConfig{
			Enabled: false,
			URL:     "/debug/vars",
		},
		Pprof:              PprofConfig{Enabled: false},
		RumConfig:          defaultRum(),
		Kibana:             defaultKibanaConfig(),
		KibanaAgentConfig:  defaultKibanaAgentConfig(),
		Aggregation:        defaultAggregationConfig(),
		Sampling:           defaultSamplingConfig(),
		Profiling:          defaultProfilingConfig(),
		DataStreams:        defaultDataStreamsConfig(),
		AgentAuth:          defaultAgentAuth(),
		JavaAttacherConfig: defaultJavaAttacherConfig(),
		WaitReadyInterval:  5 * time.Second,
	}
}
