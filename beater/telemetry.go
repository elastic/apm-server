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

package beater

import (
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var apmRegistry = monitoring.GetNamespace("state").GetRegistry().NewRegistry("apm-server")

type configTelemetry struct {
	rumEnabled           *monitoring.Bool
	apiKeysEnabled       *monitoring.Bool
	kibanaEnabled        *monitoring.Bool
	sslEnabled           *monitoring.Bool
	tailSamplingEnabled  *monitoring.Bool
	tailSamplingPolicies *monitoring.Int
}

var configMonitors = &configTelemetry{
	rumEnabled:           monitoring.NewBool(apmRegistry, "rum.enabled"),
	apiKeysEnabled:       monitoring.NewBool(apmRegistry, "api_key.enabled"),
	kibanaEnabled:        monitoring.NewBool(apmRegistry, "kibana.enabled"),
	sslEnabled:           monitoring.NewBool(apmRegistry, "ssl.enabled"),
	tailSamplingEnabled:  monitoring.NewBool(apmRegistry, "sampling.tail.enabled"),
	tailSamplingPolicies: monitoring.NewInt(apmRegistry, "sampling.tail.policies"),
}

// recordAPMServerConfig records dynamic APM Server config properties for telemetry.
// This should be called once each time runServer is called.
func recordAPMServerConfig(cfg *config.Config) {
	configMonitors.rumEnabled.Set(cfg.RumConfig.Enabled)
	configMonitors.apiKeysEnabled.Set(cfg.AgentAuth.APIKey.Enabled)
	configMonitors.kibanaEnabled.Set(cfg.Kibana.Enabled)
	configMonitors.sslEnabled.Set(cfg.TLS.IsEnabled())
	configMonitors.tailSamplingEnabled.Set(cfg.Sampling.Tail.Enabled)
	configMonitors.tailSamplingPolicies.Set(int64(len(cfg.Sampling.Tail.Policies)))
}
