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

	"gotest.tools/assert"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestRecordConfigs(t *testing.T) {
	resetCounters()
	defer resetCounters()

	info := beat.Info{Name: "apm-server", Version: "7.x"}
	apmCfg := config.DefaultConfig(info.Version)
	apmCfg.APIKeyConfig.Enabled = true
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
	recordConfigs(info, apmCfg, rootCfg)

	assert.Equal(t, configMonitors.ilmSetupEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.rumEnabled.Get(), int64(0))
	assert.Equal(t, configMonitors.apiKeysEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.kibanaEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.pipelinesEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.pipelinesOverwrite.Get(), int64(0))
	assert.Equal(t, configMonitors.setupTemplateEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.setupTemplateOverwrite.Get(), int64(1))
	assert.Equal(t, configMonitors.setupTemplateAppendFields.Get(), int64(0))
	assert.Equal(t, configMonitors.ilmEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.ilmSetupEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.ilmSetupOverwrite.Get(), int64(1))
	assert.Equal(t, configMonitors.ilmSetupRequirePolicy.Get(), int64(0))
	assert.Equal(t, configMonitors.jaegerGRPCEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.jaegerHTTPEnabled.Get(), int64(1))
	assert.Equal(t, configMonitors.sslEnabled.Get(), int64(0))
}

func resetCounters() {
	configMonitors.rumEnabled.Set(0)
	configMonitors.apiKeysEnabled.Set(0)
	configMonitors.kibanaEnabled.Set(0)
	configMonitors.jaegerHTTPEnabled.Set(0)
	configMonitors.jaegerGRPCEnabled.Set(0)
	configMonitors.sslEnabled.Set(0)
	configMonitors.pipelinesEnabled.Set(0)
	configMonitors.pipelinesOverwrite.Set(0)
	configMonitors.setupTemplateEnabled.Set(0)
	configMonitors.setupTemplateOverwrite.Set(0)
	configMonitors.setupTemplateAppendFields.Set(0)
	configMonitors.ilmSetupEnabled.Set(0)
	configMonitors.ilmSetupOverwrite.Set(0)
	configMonitors.ilmSetupRequirePolicy.Set(0)
	configMonitors.ilmEnabled.Set(0)
}
