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

	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/elasticsearch"
)

const msgInvalidConfigAgentCfg = "invalid value for `apm-server.agent.config.cache.expiration`, only accepting full seconds"

// DynamicAgentConfig configuration for dynamically querying agent configuration
// via Elasticsearch or Kibana.
type DynamicAgentConfig struct {
	ESConfig *elasticsearch.Config `config:"elasticsearch"`
	Cache    Cache                 `config:"cache"`
}

func (c *DynamicAgentConfig) setup(log *logp.Logger, outputESCfg *config.C) error {
	if float64(int(c.Cache.Expiration.Seconds())) != c.Cache.Expiration.Seconds() {
		return errors.New(msgInvalidConfigAgentCfg)
	}

	if outputESCfg == nil {
		return nil
	}
	log.Info("using Elasticsearch output config for fetching agent config")
	if err := outputESCfg.Unpack(c.ESConfig); err != nil {
		return errors.Wrap(err, "unpacking Elasticsearch config into agent config")
	}
	return nil
}

// Cache holds config information about cache expiration
type Cache struct {
	Expiration time.Duration `config:"expiration"`
}

// defaultDynamicAgentConfig holds the default DynamicAgentConfig
func defaultDynamicAgentConfig() DynamicAgentConfig {
	return DynamicAgentConfig{
		ESConfig: elasticsearch.DefaultConfig(),
		Cache:    Cache{Expiration: 30 * time.Second},
	}
}

// AgentConfig defines configuration for agents.
type AgentConfig struct {
	Service   Service `config:"service"`
	AgentName string  `config:"agent.name"`
	Etag      string  `config:"etag"`
	Config    map[string]string
}

func (s *AgentConfig) setup() error {
	if s.Config == nil {
		return errInvalidAgentConfigMissingConfig
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
