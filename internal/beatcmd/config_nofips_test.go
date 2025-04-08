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

//go:build !requirefips

package beatcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/keystore"
)

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
