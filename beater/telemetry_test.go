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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestRecordConfigs(t *testing.T) {
	resetCounters()
	defer resetCounters()

	info := beat.Info{Name: "apm-server", Version: "7.x"}
	apmCfg := config.DefaultConfig()
	apmCfg.AgentAuth.APIKey.Enabled = true
	apmCfg.Kibana.Enabled = true
	apmCfg.JaegerConfig.GRPC.Enabled = true
	apmCfg.JaegerConfig.HTTP.Enabled = true
	rootCfg := common.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"ilm": map[string]interface{}{
				"setup": map[string]interface{}{
					"enabled":        true,
					"overwrite":      true,
					"require_policy": false,
				},
			},
		},
		"setup": map[string]interface{}{
			"template": map[string]interface{}{
				"overwrite": true,
			},
		},
	})
	require.NoError(t, recordRootConfig(info, rootCfg))
	recordAPMServerConfig(apmCfg)

	assert.Equal(t, configMonitors.ilmSetupEnabled.Get(), true)
	assert.Equal(t, configMonitors.rumEnabled.Get(), false)
	assert.Equal(t, configMonitors.apiKeysEnabled.Get(), true)
	assert.Equal(t, configMonitors.kibanaEnabled.Get(), true)
	assert.Equal(t, configMonitors.setupTemplateEnabled.Get(), true)
	assert.Equal(t, configMonitors.setupTemplateOverwrite.Get(), true)
	assert.Equal(t, configMonitors.setupTemplateAppendFields.Get(), false)
	assert.Equal(t, configMonitors.ilmEnabled.Get(), true)
	assert.Equal(t, configMonitors.ilmSetupEnabled.Get(), true)
	assert.Equal(t, configMonitors.ilmSetupOverwrite.Get(), true)
	assert.Equal(t, configMonitors.ilmSetupRequirePolicy.Get(), false)
	assert.Equal(t, configMonitors.jaegerGRPCEnabled.Get(), true)
	assert.Equal(t, configMonitors.jaegerHTTPEnabled.Get(), true)
	assert.Equal(t, configMonitors.sslEnabled.Get(), false)
}

func resetCounters() {
	configMonitors.rumEnabled.Set(false)
	configMonitors.apiKeysEnabled.Set(false)
	configMonitors.kibanaEnabled.Set(false)
	configMonitors.jaegerHTTPEnabled.Set(false)
	configMonitors.jaegerGRPCEnabled.Set(false)
	configMonitors.sslEnabled.Set(false)
	configMonitors.setupTemplateEnabled.Set(false)
	configMonitors.setupTemplateOverwrite.Set(false)
	configMonitors.setupTemplateAppendFields.Set(false)
	configMonitors.ilmSetupEnabled.Set(false)
	configMonitors.ilmSetupOverwrite.Set(false)
	configMonitors.ilmSetupRequirePolicy.Set(false)
	configMonitors.ilmEnabled.Set(false)
}
