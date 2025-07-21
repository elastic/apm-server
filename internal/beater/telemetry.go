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
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

// recordAPMServerConfig records dynamic APM Server config properties for telemetry.
// This should be called once each time runServer is called.
func recordAPMServerConfig(cfg *config.Config, stateRegistry *monitoring.Registry) {
	apmRegistry := stateRegistry.NewRegistry("apm-server")
	monitoring.NewBool(apmRegistry, "rum.enabled").Set(cfg.RumConfig.Enabled)
	monitoring.NewBool(apmRegistry, "api_key.enabled").Set(cfg.AgentAuth.APIKey.Enabled)
	monitoring.NewBool(apmRegistry, "kibana.enabled").Set(cfg.Kibana.Enabled)
	monitoring.NewBool(apmRegistry, "ssl.enabled").Set(cfg.TLS.IsEnabled())
	monitoring.NewBool(apmRegistry, "sampling.tail.enabled").Set(cfg.Sampling.Tail.Enabled)
	monitoring.NewInt(apmRegistry, "sampling.tail.policies").Set(int64(len(cfg.Sampling.Tail.Policies)))
}
