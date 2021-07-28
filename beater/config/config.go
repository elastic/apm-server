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

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const (
	// DefaultPort of APM Server
	DefaultPort = "8200"

	msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"
)

var (
	errInvalidAgentConfigMissingConfig = errors.New("agent_config: no config set")
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
	SelfInstrumentation       InstrumentationConfig   `config:"instrumentation"`
	RumConfig                 RumConfig               `config:"rum"`
	Register                  RegisterConfig          `config:"register"`
	Mode                      Mode                    `config:"mode"`
	Kibana                    KibanaConfig            `config:"kibana"`
	KibanaAgentConfig         KibanaAgentConfig       `config:"agent.config"`
	JaegerConfig              JaegerConfig            `config:"jaeger"`
	Aggregation               AggregationConfig       `config:"aggregation"`
	Sampling                  SamplingConfig          `config:"sampling"`
	DataStreams               DataStreamsConfig       `config:"data_streams"`
	DefaultServiceEnvironment string                  `config:"default_service_environment"`
	JavaAttacherConfig        JavaAttacherConfig      `config:"java_attacher"`

	Pipeline string

	AgentConfigs []AgentConfig `config:"agent_config"`
}

// NewConfig creates a Config struct based on the default config and the given input params
func NewConfig(ucfg *common.Config, outputESCfg *common.Config) (*Config, error) {
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

	if err := setDeprecatedConfig(c, ucfg, logger); err != nil {
		return nil, err
	}

	if err := c.RumConfig.setup(logger, c.DataStreams.Enabled, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.AgentAuth.APIKey.setup(logger, outputESCfg); err != nil {
		return nil, err
	}

	if err := c.JaegerConfig.setup(c); err != nil {
		return nil, err
	}

	if err := c.Sampling.Tail.setup(logger, outputESCfg); err != nil {
		return nil, err
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

	if c.DataStreams.Enabled || (outputESCfg != nil && (outputESCfg.HasField("pipeline") || outputESCfg.HasField("pipelines"))) {
		c.Pipeline = ""
	}
	return c, nil
}

// setDeprecatedConfig translates deprecated top-level config attributes to the
// current config structure.
func setDeprecatedConfig(out *Config, in *common.Config, logger *logp.Logger) error {
	var deprecatedConfig struct {
		APIKey      APIKeyAgentAuth `config:"api_key"`
		SecretToken string          `config:"secret_token"`
	}
	deprecatedConfig.APIKey = defaultAPIKeyAgentAuth()
	if err := in.Unpack(&deprecatedConfig); err != nil {
		return err
	}

	warnIgnored := func(deprecated, replacement string) {
		logger.Warnf("ignoring deprecated config %q as %q is defined", deprecated, replacement)
	}
	if deprecatedConfig.APIKey.configured {
		// "apm-server.api_key" -> "apm-server.auth.api_key"
		if out.AgentAuth.APIKey.configured {
			warnIgnored("apm-server.api_key", "apm-server.auth.api_key")
		} else {
			out.AgentAuth.APIKey = deprecatedConfig.APIKey
		}
	}
	if deprecatedConfig.SecretToken != "" {
		// "apm-server.secret_token" -> "apm-server.auth.secret_token"
		if out.AgentAuth.SecretToken != "" {
			warnIgnored("apm-server.secret_token", "apm-server.auth.secret_token")
		} else {
			out.AgentAuth.SecretToken = deprecatedConfig.SecretToken
		}
	}
	return nil
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
		Expvar: ExpvarConfig{
			Enabled: false,
			URL:     "/debug/vars",
		},
		Pprof:               PprofConfig{Enabled: false},
		SelfInstrumentation: defaultInstrumentationConfig(),
		RumConfig:           defaultRum(),
		Register:            defaultRegisterConfig(),
		Mode:                ModeProduction,
		Kibana:              defaultKibanaConfig(),
		KibanaAgentConfig:   defaultKibanaAgentConfig(),
		Pipeline:            defaultAPMPipeline,
		JaegerConfig:        defaultJaeger(),
		Aggregation:         defaultAggregationConfig(),
		Sampling:            defaultSamplingConfig(),
		DataStreams:         defaultDataStreamsConfig(),
		AgentAuth:           defaultAgentAuth(),
		JavaAttacherConfig:  defaultJavaAttacherConfig(),
	}
}
