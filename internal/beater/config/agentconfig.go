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
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-ucfg"
)

const msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"

// esCredentialsConfig is exposed to allow fine-tuned permission control
// and is required when working with Elastic Agent standalone or Fleet.
type esCredentialsConfig struct {
	Username string `config:"username"`
	Password string `config:"password"`
	APIKey   string `config:"api_key"`
}

func (c *esCredentialsConfig) Unpack(in **elasticsearch.Config) error {
	(*in).Username = c.Username
	(*in).Password = c.Password
	(*in).APIKey = c.APIKey
	return nil
}

// AgentConfig configuration for dynamically querying agent configuration
// via Elasticsearch or Kibana.
type AgentConfig struct {
	ESConfig *elasticsearch.Config
	Cache    Cache `config:"cache"`

	es *esCredentialsConfig
}

func (c *AgentConfig) Unpack(in *config.C) error {
	type agentConfig AgentConfig
	cfg := agentConfig(defaultAgentConfig())
	if err := in.Unpack(&cfg); err != nil {
		return errors.Wrap(err, "error unpacking config")
	}
	*c = AgentConfig(cfg)

	es, err := in.Child("elasticsearch", -1)
	if err == nil {
		err = es.Unpack(&c.es)
	}

	var ucfgError ucfg.Error
	if !errors.As(err, &ucfgError) || ucfgError.Reason() != ucfg.ErrMissing {
		return err
	}
	return nil
}

func (c *AgentConfig) setup(log *logp.Logger, outputESCfg *config.C) error {
	if float64(int(c.Cache.Expiration.Seconds())) != c.Cache.Expiration.Seconds() {
		return errors.New(msgInvalidConfigAgentCfg)
	}
	if outputESCfg != nil {
		log.Info("using Elasticsearch output config for fetching agent config")
		if err := outputESCfg.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "unpacking Elasticsearch output config into agent config")
		}
	}
	if c.es != nil {
		if err := c.es.Unpack(&c.ESConfig); err != nil {
			return errors.Wrap(err, "unpacking user supplied Elasticsearch config into agent config")
		}
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

// FleetAgentConfig defines configuration for agents.
type FleetAgentConfig struct {
	Service   Service `config:"service"`
	AgentName string  `config:"agent.name"`
	Etag      string  `config:"etag"`
	Config    map[string]string
}

func (s *FleetAgentConfig) setup() error {
	if s.Config == nil {
		// Config may be passed to APM Server as `null` when no attributes
		// are defined in an APM Agent central configuration entry.
		s.Config = make(map[string]string)
	}
	if s.Etag == "" {
		m, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("error generating etag for %s: %v", s.Service, err)
		}
		s.Etag = fmt.Sprintf("%x", md5.Sum(m))
	}
	return nil
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
