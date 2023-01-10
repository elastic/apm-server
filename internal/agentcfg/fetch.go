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

	"github.com/elastic/apm-server/internal/beater/config"
)

// TransactionSamplingRateKey is the agent configuration key for the
// sampling rate. This is used by the Jaeger handler to adapt our agent
// configuration to the Jaeger remote sampler protocol.
const TransactionSamplingRateKey = "transaction_sample_rate"

// Fetcher defines a common interface to retrieving agent config.
type Fetcher interface {
	Fetch(context.Context, Query) (Result, error)
}

// DirectFetcher is an agent config fetcher which serves requests out of a
// statically defined set of agent configuration. These configurations are
// typically provided via Fleet.
type DirectFetcher struct {
	cfgs []AgentConfig
}

// NewDirectFetcher returns a new DirectFetcher that serves agent configuration
// requests using cfgs.
func NewDirectFetcher(cfgs []AgentConfig) *DirectFetcher {
	return &DirectFetcher{cfgs}
}

// Fetch finds a matching AgentConfig in cfgs based on the received Query.
func (f *DirectFetcher) Fetch(_ context.Context, query Query) (Result, error) {
	return matchAgentConfig(query, f.cfgs), nil
}

// matchAgentConfig finds a matching AgentConfig based on the received Query.
// Order of precedence:
// - service.name and service.environment match an AgentConfig
// - service.name matches an AgentConfig, service.environment == ""
// - service.environment matches an AgentConfig, service.name == ""
// - an AgentConfig without a name or environment set
// Return an empty result if no matching result is found.
func matchAgentConfig(query Query, cfgs []AgentConfig) Result {
	name, env := query.Service.Name, query.Service.Environment
	result := zeroResult()
	var nameConf, envConf, defaultConf *AgentConfig

	for i, cfg := range cfgs {
		if cfg.ServiceName == name && cfg.ServiceEnvironment == env {
			nameConf = &cfgs[i]
			break
		} else if cfg.ServiceName == name && cfg.ServiceEnvironment == "" {
			nameConf = &cfgs[i]
		} else if cfg.ServiceName == "" && cfg.ServiceEnvironment == env {
			envConf = &cfgs[i]
		} else if cfg.ServiceName == "" && cfg.ServiceEnvironment == "" {
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

// AgentConfig holds an agent configuration definition.
type AgentConfig struct {
	// ServiceName holds the service name to which this agent configuration
	// applies. This is optional.
	ServiceName string

	// ServiceEnvironment holds the service environment to which this agent
	// configuration applies. This is optional.
	ServiceEnvironment string

	// AgentName holds the agent name to which this agent configuration
	// applies. This is optional, and is used for filtering configuration
	// settings for unauthenticated agents.
	AgentName string

	// Etag holds a unique ID for the configuration, which agents
	// will send along with their queries. The server uses this to
	// determine whether agent configuration has been applied.
	Etag string

	// Config holds configuration settings that should be sent to
	// agents matching the above constraints.
	Config map[string]string
}

func ConvertAgentConfigs(fleetAgentConfigs []config.FleetAgentConfig) []AgentConfig {
	agentConfigurations := make([]AgentConfig, len(fleetAgentConfigs))
	for i, in := range fleetAgentConfigs {
		agentConfigurations[i] = AgentConfig{
			ServiceName:        in.Service.Name,
			ServiceEnvironment: in.Service.Environment,
			AgentName:          in.AgentName,
			Etag:               in.Etag,
			Config:             in.Config,
		}
	}
	return agentConfigurations
}
