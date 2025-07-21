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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-ucfg"
)

const msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"

// AgentConfig configuration for dynamically querying agent configuration
// via Elasticsearch or Kibana.
type AgentConfig struct {
	ESConfig *elasticsearch.Config
	Cache    Cache `config:"cache"`

	ESOverrideConfigured bool
	es                   *config.C
}

func (c *AgentConfig) Unpack(in *config.C) error {
	type agentConfig AgentConfig
	cfg := agentConfig(defaultAgentConfig())
	if err := in.Unpack(&cfg); err != nil {
		return fmt.Errorf("error unpacking agent config: %w", err)
	}
	*c = AgentConfig(cfg)

	var err error
	c.es, err = in.Child("elasticsearch", -1)

	if err != nil {
		var ucfgError ucfg.Error
		if !errors.As(err, &ucfgError) || ucfgError.Reason() != ucfg.ErrMissing {
			return fmt.Errorf("error unpacking agent elasticsearch config: %w", err)
		}
	}
	return nil
}

func (c *AgentConfig) setup(log *logp.Logger, outputESCfg *config.C) error {
	if c.Cache.Expiration%time.Second != 0 {
		return errors.New(msgInvalidConfigAgentCfg)
	}
	if outputESCfg != nil {
		log.Info("using output.elasticsearch for fetching agent config")
		if err := outputESCfg.Unpack(&c.ESConfig); err != nil {
			return fmt.Errorf("error unpacking output.elasticsearch for fetching agent config: %w", err)
		}
	}
	if c.es != nil {
		log.Info("using apm-server.agent.config.elasticsearch for fetching agent config")
		c.ESOverrideConfigured = true

		// Empty out credential fields before merging if credentials are provided in agentcfg ES config
		if c.es.HasField("api_key") || c.es.HasField("username") || c.es.HasField("password") {
			c.ESConfig.APIKey = ""
			c.ESConfig.Username = ""
			c.ESConfig.Password = ""
		}

		if err := c.es.Unpack(c.ESConfig); err != nil {
			return fmt.Errorf("error unpacking apm-server.agent.config.elasticsearch for fetching agent config: %w", err)
		}
		c.es = nil
	}
	return nil
}

// Cache holds config information about cache expiration
type Cache struct {
	Expiration time.Duration `config:"expiration" validate:"min=1s"`
}

// defaultAgentConfig holds the default AgentConfig
func defaultAgentConfig() AgentConfig {
	return AgentConfig{
		ESConfig: elasticsearch.DefaultConfig(),
		Cache: Cache{
			Expiration: 30 * time.Second,
		},
	}
}

// Service defines a unique way of identifying a running agent.
type Service struct {
	Name        string `config:"name"`
	Environment string `config:"environment"`
}

// String implements the Stringer interface.
func (s *Service) String() string {
	var name, env string
	if s.Name != "" {
		name = "service.name=" + s.Name
	}
	if s.Environment != "" {
		env = "service.environment=" + s.Environment
	}
	return strings.Join([]string{name, env}, " ")
}
