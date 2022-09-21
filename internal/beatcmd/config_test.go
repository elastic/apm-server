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

package beatcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/keystore"
	"github.com/elastic/elastic-agent-libs/paths"
)

func TestLoadConfig(t *testing.T) {
	initCfgfile(t, `
apm-server:
  host: :8200
  `)
	cfg, rawConfig, keystore, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, rawConfig)
	assert.NotNil(t, keystore)

	assertConfigEqual(t, map[string]interface{}{
		"apm-server": map[string]interface{}{
			"host": ":8200",
		},
		"path": map[string]interface{}{
			"config": paths.Paths.Config,
			"logs":   paths.Paths.Logs,
			"data":   paths.Paths.Data,
			"home":   paths.Paths.Home,
		},
	}, rawConfig)

	assertConfigEqual(t, map[string]interface{}{"host": ":8200"}, cfg.APMServer)
}

func TestLoadConfigMerge(t *testing.T) {
	initCfgfile(t, `
apm-server:
  host: :8200
  `)
	cfg, _, _, err := LoadConfig(WithMergeConfig(
		config.MustNewConfigFrom("apm-server.host: localhost:8200"),
		config.MustNewConfigFrom("apm-server.shutdown_timeout: 1s"),
	))
	require.NoError(t, err)

	assertConfigEqual(t, map[string]interface{}{
		"host":             "localhost:8200",
		"shutdown_timeout": "1s",
	}, cfg.APMServer)
}

func TestLoadConfigKeystore(t *testing.T) {
	initCfgfile(t, `
apm-server:
  auth.secret_token: ${APM_SECRET_TOKEN}
  `)

	cfg, _, _, err := LoadConfig(WithDisableConfigResolution())
	require.NoError(t, err)
	assertConfigEqual(t, map[string]interface{}{
		"auth": map[string]interface{}{
			"secret_token": "${APM_SECRET_TOKEN}",
		},
	}, cfg.APMServer)

	cfg, _, ks, err := LoadConfig()
	require.NoError(t, err)

	err = cfg.APMServer.Unpack(new(map[string]interface{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `missing field accessing 'apm-server.auth'`)

	wks, err := keystore.AsWritableKeystore(ks)
	require.NoError(t, err)
	err = wks.Store("APM_SECRET_TOKEN", []byte("abc123"))
	require.NoError(t, err)
	err = wks.Save()
	require.NoError(t, err)

	assertConfigEqual(t, map[string]interface{}{
		"auth": map[string]interface{}{
			"secret_token": "abc123",
		},
	}, cfg.APMServer)
}

func assertConfigEqual(t testing.TB, expected map[string]interface{}, actual *config.C) {
	t.Helper()
	var m map[string]interface{}
	err := actual.Unpack(&m)
	require.NoError(t, err)
	assert.Equal(t, expected, m)
}

func TestLoadConfigProcessorsDisallowed(t *testing.T) {
	initCfgfile(t, `
processors:
  add_cloud_metadata: {}
`)

	_, _, _, err := LoadConfig()
	assert.EqualError(t, err, "invalid config: libbeat processors are not supported")
}

func initCfgfile(t testing.TB, content string) (home string) {
	home = t.TempDir()
	content += "\npath.home: " + home

	origConfigPath := cfgfile.GetPathConfig()
	origConfigFile := strings.TrimSuffix(cfgfile.GetDefaultCfgfile(), ".yml")
	t.Cleanup(func() {
		cfgfile.SetConfigPath(origConfigPath)
		cfgfile.ChangeDefaultCfgfileFlag(origConfigFile)
	})

	configFile := filepath.Join(home, "testing.yml")
	err := os.WriteFile(configFile, []byte(strings.TrimSpace(content)), 0644)
	require.NoError(t, err)
	cfgfile.SetConfigPath(home)
	cfgfile.ChangeDefaultCfgfileFlag("testing")
	return home
}
