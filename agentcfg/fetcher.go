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

package agentcfg

import (
	"context"

	"github.com/elastic/apm-server/beater/config"
)

const (
	TransactionSamplingRateKey = "transaction_sample_rate"
)

// Fetcher defines a common interface to retrieving agent config.
type Fetcher interface {
	Fetch(context.Context, Query) (Result, error)
}

type DirectFetcher struct {
	cfgs []config.AgentConfig
}

func NewDirectFetcher(cfgs []config.AgentConfig) *DirectFetcher {
	return &DirectFetcher{cfgs}
}

// Fetch finds a matching AgentConfig based on the received Query.
// Order of precedence:
// - service.name and service.environment match an AgentConfig
// - service.name matches an AgentConfig, service.environment == ""
// - service.environment matches an AgentConfig, service.name == ""
// - an AgentConfig without a name or environment set
// Return an empty result if no matching result is found.
func (f *DirectFetcher) Fetch(_ context.Context, query Query) (Result, error) {
	return matchAgentConfig(query, f.cfgs), nil
}

func matchAgentConfig(query Query, cfgs []config.AgentConfig) Result {
	name, env := query.Service.Name, query.Service.Environment
	result := zeroResult()
	var nameConf, envConf, defaultConf *config.AgentConfig

	for i, cfg := range cfgs {
		if cfg.Service.Name == name && cfg.Service.Environment == env {
			nameConf = &cfgs[i]
			break
		} else if cfg.Service.Name == name && cfg.Service.Environment == "" {
			nameConf = &cfgs[i]
		} else if cfg.Service.Name == "" && cfg.Service.Environment == env {
			envConf = &cfgs[i]
		} else if cfg.Service.Name == "" && cfg.Service.Environment == "" {
			defaultConf = &cfgs[i]
		}
	}

	if nameConf != nil {
		result = Result{Source{
			Settings: nameConf.Config,
			Etag:     nameConf.Etag,
			Agent:    nameConf.AgentName,
		}}
	} else if envConf != nil {
		result = Result{Source{
			Settings: envConf.Config,
			Etag:     envConf.Etag,
			Agent:    envConf.AgentName,
		}}
	} else if defaultConf != nil {
		result = Result{Source{
			Settings: defaultConf.Config,
			Etag:     defaultConf.Etag,
			Agent:    defaultConf.AgentName,
		}}
	}
	return result
}
