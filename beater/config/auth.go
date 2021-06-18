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
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
)

// AgentAuth holds config related to agent auth.
type AgentAuth struct {
	APIKey      APIKeyAgentAuth `config:"api_key"`
	SecretToken string          `config:"secret_token"`
}

// APIKeyAgentAuth holds config related to API Key auth for agents.
type APIKeyAgentAuth struct {
	Enabled     bool                  `config:"enabled"`
	LimitPerMin int                   `config:"limit"`
	ESConfig    *elasticsearch.Config `config:"elasticsearch"`

	configured   bool // api_key explicitly defined
	esConfigured bool // api_key.elasticsearch explicitly defined
}

func (a *APIKeyAgentAuth) Unpack(in *common.Config) error {
	type underlyingAPIKeyAgentAuth APIKeyAgentAuth
	if err := in.Unpack((*underlyingAPIKeyAgentAuth)(a)); err != nil {
		return errors.Wrap(err, "error unpacking api_key config")
	}
	a.configured = true
	a.esConfigured = in.HasField("elasticsearch")
	return nil
}

func (a *APIKeyAgentAuth) setup(log *logp.Logger, outputESCfg *common.Config) error {
	if !a.Enabled || a.esConfigured || outputESCfg == nil {
		return nil
	}
	log.Info("Falling back to elasticsearch output for API Key usage")
	if err := outputESCfg.Unpack(&a.ESConfig); err != nil {
		return errors.Wrap(err, "unpacking Elasticsearch config into API key config")
	}
	return nil
}

func defaultAgentAuth() AgentAuth {
	return AgentAuth{
		APIKey: defaultAPIKeyAgentAuth(),
	}
}

func defaultAPIKeyAgentAuth() APIKeyAgentAuth {
	return APIKeyAgentAuth{
		Enabled:     false,
		LimitPerMin: 100,
		ESConfig:    elasticsearch.DefaultConfig(),
	}
}
