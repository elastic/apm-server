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
	"github.com/elastic/beats/v7/libbeat/idxmgmt/ilm"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var apmRegistry = monitoring.GetNamespace("state").GetRegistry().NewRegistry("apm-server")
var configRegistry = apmRegistry.NewRegistry("configuration")

type configTelemetry struct {
	rumEnabled                *monitoring.Int
	apiKeysEnabled            *monitoring.Int
	kibanaEnabled             *monitoring.Int
	pipelinesEnabled          *monitoring.Int
	pipelinesOverwrite        *monitoring.Int
	setupTemplateEnabled      *monitoring.Int
	setupTemplateOverwrite    *monitoring.Int
	setupTemplateAppendFields *monitoring.Int
	ilmEnabled                *monitoring.Int
	ilmSetupEnabled           *monitoring.Int
	ilmSetupOverwrite         *monitoring.Int
	ilmSetupRequirePolicy     *monitoring.Int
	jaegerGRPCEnabled         *monitoring.Int
	jaegerHTTPEnabled         *monitoring.Int
	sslEnabled                *monitoring.Int
}

var configMonitors = &configTelemetry{
	rumEnabled:                monitoring.NewInt(configRegistry, "rumEnabled"),
	apiKeysEnabled:            monitoring.NewInt(configRegistry, "apiKeysEnabled"),
	kibanaEnabled:             monitoring.NewInt(configRegistry, "kibanaEnabled"),
	pipelinesEnabled:          monitoring.NewInt(configRegistry, "pipelinesEnabled"),
	pipelinesOverwrite:        monitoring.NewInt(configRegistry, "pipelineOverwrite"),
	setupTemplateEnabled:      monitoring.NewInt(configRegistry, "setupTemplateEnabled"),
	setupTemplateOverwrite:    monitoring.NewInt(configRegistry, "setupTemplateOverwrite"),
	setupTemplateAppendFields: monitoring.NewInt(configRegistry, "setupTemplateAppendFields"),
	ilmEnabled:                monitoring.NewInt(configRegistry, "ilmEnabled"),
	ilmSetupEnabled:           monitoring.NewInt(configRegistry, "ilmSetupEnabled"),
	ilmSetupOverwrite:         monitoring.NewInt(configRegistry, "ilmSetupOverwrite"),
	ilmSetupRequirePolicy:     monitoring.NewInt(configRegistry, "ilmSetupRequirePolicy"),
	jaegerGRPCEnabled:         monitoring.NewInt(configRegistry, "jaegerGRPCEnabled"),
	jaegerHTTPEnabled:         monitoring.NewInt(configRegistry, "jaegerHTTPEnabled"),
	sslEnabled:                monitoring.NewInt(configRegistry, "sslEnabled"),
}

func recordConfigs(info beat.Info, apmCfg *config.Config, rootCfg *common.Config) {
	if apmCfg.RumConfig.IsEnabled() {
		configMonitors.rumEnabled.Inc()
	}
	if apmCfg.APIKeyConfig.IsEnabled() {
		configMonitors.apiKeysEnabled.Inc()
	}
	if apmCfg.Kibana.Enabled {
		configMonitors.kibanaEnabled.Inc()
	}
	if apmCfg.JaegerConfig.HTTP.Enabled {
		configMonitors.jaegerHTTPEnabled.Inc()
	}
	if apmCfg.JaegerConfig.GRPC.Enabled {
		configMonitors.jaegerGRPCEnabled.Inc()
	}
	if apmCfg.TLS.IsEnabled() {
		configMonitors.sslEnabled.Inc()
	}
	if apmCfg.Register.Ingest.Pipeline.IsEnabled() {
		configMonitors.pipelinesEnabled.Inc()
	}
	if apmCfg.Register.Ingest.Pipeline.ShouldOverwrite() {
		configMonitors.pipelinesOverwrite.Inc()
	}
	indexManagementCfg, err := idxmgmt.NewIndexManagementConfig(info, rootCfg)
	if err != nil {
		return
	}
	if indexManagementCfg.Template.Enabled {
		configMonitors.setupTemplateEnabled.Inc()
	}
	if indexManagementCfg.Template.Overwrite {
		configMonitors.setupTemplateOverwrite.Inc()
	}
	if len(indexManagementCfg.Template.AppendFields.GetKeys()) > 0 {
		configMonitors.setupTemplateAppendFields.Inc()
	}
	if indexManagementCfg.ILM.Setup.Enabled {
		configMonitors.ilmSetupEnabled.Inc()
	}
	if indexManagementCfg.ILM.Setup.Overwrite {
		configMonitors.ilmSetupOverwrite.Inc()
	}
	if indexManagementCfg.ILM.Setup.RequirePolicy {
		configMonitors.ilmSetupRequirePolicy.Inc()
	}
	if mode := indexManagementCfg.ILM.Mode; mode == ilm.ModeAuto || mode == ilm.ModeEnabled {
		configMonitors.ilmEnabled.Inc()
	}
}
