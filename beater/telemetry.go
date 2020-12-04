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

type configTelemetry struct {
	dataStreamsEnabled        *monitoring.Bool
	rumEnabled                *monitoring.Bool
	apiKeysEnabled            *monitoring.Bool
	kibanaEnabled             *monitoring.Bool
	pipelinesEnabled          *monitoring.Bool
	pipelinesOverwrite        *monitoring.Bool
	setupTemplateEnabled      *monitoring.Bool
	setupTemplateOverwrite    *monitoring.Bool
	setupTemplateAppendFields *monitoring.Bool
	ilmEnabled                *monitoring.Bool
	ilmSetupEnabled           *monitoring.Bool
	ilmSetupOverwrite         *monitoring.Bool
	ilmSetupRequirePolicy     *monitoring.Bool
	jaegerGRPCEnabled         *monitoring.Bool
	jaegerHTTPEnabled         *monitoring.Bool
	sslEnabled                *monitoring.Bool
	tailSamplingEnabled       *monitoring.Bool
	tailSamplingPolicies      *monitoring.Int
}

var configMonitors = &configTelemetry{
	dataStreamsEnabled:        monitoring.NewBool(apmRegistry, "data_streams.enabled"),
	rumEnabled:                monitoring.NewBool(apmRegistry, "rum.enabled"),
	apiKeysEnabled:            monitoring.NewBool(apmRegistry, "api_key.enabled"),
	kibanaEnabled:             monitoring.NewBool(apmRegistry, "kibana.enabled"),
	pipelinesEnabled:          monitoring.NewBool(apmRegistry, "register.ingest.pipeline.enabled"),
	pipelinesOverwrite:        monitoring.NewBool(apmRegistry, "register.ingest.pipeline.overwrite"),
	setupTemplateEnabled:      monitoring.NewBool(apmRegistry, "setup.template.enabled"),
	setupTemplateOverwrite:    monitoring.NewBool(apmRegistry, "setup.template.overwrite"),
	setupTemplateAppendFields: monitoring.NewBool(apmRegistry, "setup.template.append_fields"),
	ilmEnabled:                monitoring.NewBool(apmRegistry, "ilm.enabled"),
	ilmSetupEnabled:           monitoring.NewBool(apmRegistry, "ilm.setup.enabled"),
	ilmSetupOverwrite:         monitoring.NewBool(apmRegistry, "ilm.setup.overwrite"),
	ilmSetupRequirePolicy:     monitoring.NewBool(apmRegistry, "ilm.setup.require_policy"),
	jaegerGRPCEnabled:         monitoring.NewBool(apmRegistry, "jaeger.grpc.enabled"),
	jaegerHTTPEnabled:         monitoring.NewBool(apmRegistry, "jaeger.http.enabled"),
	sslEnabled:                monitoring.NewBool(apmRegistry, "ssl.enabled"),
	tailSamplingEnabled:       monitoring.NewBool(apmRegistry, "sampling.tail.enabled"),
	tailSamplingPolicies:      monitoring.NewInt(apmRegistry, "sampling.tail.policies"),
}

func recordConfigs(info beat.Info, apmCfg *config.Config, rootCfg *common.Config) error {
	indexManagementCfg, err := idxmgmt.NewIndexManagementConfig(info, rootCfg)
	if err != nil {
		return err
	}
	configMonitors.dataStreamsEnabled.Set(apmCfg.DataStreams.Enabled)
	configMonitors.rumEnabled.Set(apmCfg.RumConfig.IsEnabled())
	configMonitors.apiKeysEnabled.Set(apmCfg.APIKeyConfig.IsEnabled())
	configMonitors.kibanaEnabled.Set(apmCfg.Kibana.Enabled)
	configMonitors.jaegerHTTPEnabled.Set(apmCfg.JaegerConfig.HTTP.Enabled)
	configMonitors.jaegerGRPCEnabled.Set(apmCfg.JaegerConfig.GRPC.Enabled)
	configMonitors.sslEnabled.Set(apmCfg.TLS.IsEnabled())
	configMonitors.pipelinesEnabled.Set(apmCfg.Register.Ingest.Pipeline.IsEnabled())
	configMonitors.pipelinesOverwrite.Set(apmCfg.Register.Ingest.Pipeline.ShouldOverwrite())
	configMonitors.setupTemplateEnabled.Set(indexManagementCfg.Template.Enabled)
	configMonitors.setupTemplateOverwrite.Set(indexManagementCfg.Template.Overwrite)
	configMonitors.setupTemplateAppendFields.Set(len(indexManagementCfg.Template.AppendFields.GetKeys()) > 0)
	configMonitors.ilmSetupEnabled.Set(indexManagementCfg.ILM.Setup.Enabled)
	configMonitors.ilmSetupOverwrite.Set(indexManagementCfg.ILM.Setup.Overwrite)
	configMonitors.ilmSetupRequirePolicy.Set(indexManagementCfg.ILM.Setup.RequirePolicy)
	mode := indexManagementCfg.ILM.Mode
	configMonitors.ilmEnabled.Set(mode == ilm.ModeAuto || mode == ilm.ModeEnabled)

	tailSamplingEnabled := apmCfg.Sampling.Tail != nil && apmCfg.Sampling.Tail.Enabled
	configMonitors.tailSamplingEnabled.Set(tailSamplingEnabled)
	if tailSamplingEnabled {
		configMonitors.tailSamplingPolicies.Set(int64(len(apmCfg.Sampling.Tail.Policies)))
	}
	return nil
}
