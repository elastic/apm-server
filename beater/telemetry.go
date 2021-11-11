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
	"github.com/elastic/apm-server/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var apmRegistry = monitoring.GetNamespace("state").GetRegistry().NewRegistry("apm-server")

type configTelemetry struct {
	dataStreamsEnabled        *monitoring.Bool
	rumEnabled                *monitoring.Bool
	apiKeysEnabled            *monitoring.Bool
	kibanaEnabled             *monitoring.Bool
	setupTemplateEnabled      *monitoring.Bool
	setupTemplateOverwrite    *monitoring.Bool
	setupTemplateAppendFields *monitoring.Bool
	ilmEnabled                *monitoring.Bool
	ilmSetupEnabled           *monitoring.Bool
	ilmSetupOverwrite         *monitoring.Bool
	ilmSetupRequirePolicy     *monitoring.Bool
	sslEnabled                *monitoring.Bool
	tailSamplingEnabled       *monitoring.Bool
	tailSamplingPolicies      *monitoring.Int
}

var configMonitors = &configTelemetry{
	dataStreamsEnabled:        monitoring.NewBool(apmRegistry, "data_streams.enabled"),
	rumEnabled:                monitoring.NewBool(apmRegistry, "rum.enabled"),
	apiKeysEnabled:            monitoring.NewBool(apmRegistry, "api_key.enabled"),
	kibanaEnabled:             monitoring.NewBool(apmRegistry, "kibana.enabled"),
	setupTemplateEnabled:      monitoring.NewBool(apmRegistry, "setup.template.enabled"),
	setupTemplateOverwrite:    monitoring.NewBool(apmRegistry, "setup.template.overwrite"),
	setupTemplateAppendFields: monitoring.NewBool(apmRegistry, "setup.template.append_fields"),
	ilmEnabled:                monitoring.NewBool(apmRegistry, "ilm.enabled"),
	ilmSetupEnabled:           monitoring.NewBool(apmRegistry, "ilm.setup.enabled"),
	ilmSetupOverwrite:         monitoring.NewBool(apmRegistry, "ilm.setup.overwrite"),
	ilmSetupRequirePolicy:     monitoring.NewBool(apmRegistry, "ilm.setup.require_policy"),
	sslEnabled:                monitoring.NewBool(apmRegistry, "ssl.enabled"),
	tailSamplingEnabled:       monitoring.NewBool(apmRegistry, "sampling.tail.enabled"),
	tailSamplingPolicies:      monitoring.NewInt(apmRegistry, "sampling.tail.policies"),
}

// recordRootConfig records static properties of the given root config for telemetry.
// This should be called once at startup, with the root config.
func recordRootConfig(info beat.Info, rootCfg *common.Config) error {
	indexManagementCfg, err := idxmgmt.NewIndexManagementConfig(info, rootCfg)
	if err != nil {
		return err
	}
	configMonitors.dataStreamsEnabled.Set(indexManagementCfg.DataStreams)
	configMonitors.setupTemplateEnabled.Set(indexManagementCfg.Template.Enabled)
	configMonitors.setupTemplateOverwrite.Set(indexManagementCfg.Template.Overwrite)
	configMonitors.setupTemplateAppendFields.Set(len(indexManagementCfg.Template.AppendFields.GetKeys()) > 0)
	configMonitors.ilmSetupEnabled.Set(indexManagementCfg.ILM.Setup.Enabled)
	configMonitors.ilmSetupOverwrite.Set(indexManagementCfg.ILM.Setup.Overwrite)
	configMonitors.ilmSetupRequirePolicy.Set(indexManagementCfg.ILM.Setup.RequirePolicy)
	configMonitors.ilmEnabled.Set(indexManagementCfg.ILM.Enabled)
	return nil
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
