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

package systemtest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func apiKeyCommand(subcommand string, args ...string) *apmservertest.ServerCmd {
	cfg := apmservertest.DefaultConfig()
	cfgargs, err := cfg.Args()
	if err != nil {
		panic(err)
	}

	var esargs []string
	for i := 1; i < len(cfgargs); i += 2 {
		if !strings.HasPrefix(cfgargs[i], "output.elasticsearch") {
			continue
		}
		esargs = append(esargs, "-E", cfgargs[i])
	}

	userargs := args
	args = append([]string{subcommand}, esargs...)
	args = append(args, userargs...)
	return apmservertest.ServerCommand("apikey", args...)
}

func TestAPIKeyCreate(t *testing.T) {
	cmd := apiKeyCommand("create", "--name", t.Name(), "--json")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(out, &m)
	require.NoError(t, err)
	assert.Equal(t, t.Name(), m["name"])
	assert.Contains(t, m, "id")
	assert.Contains(t, m, "api_key")
	assert.Contains(t, m, "credentials")
}

func TestAPIKeyCreateExpiration(t *testing.T) {
	cmd := apiKeyCommand("create", "--name", t.Name(), "--json", "--expiration=1d")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(out, &m)
	require.NoError(t, err)
	assert.Contains(t, m, "expiration")
}

func TestAPIKeyInvalidateName(t *testing.T) {
	for i := 0; i < 2; i++ {
		cmd := apiKeyCommand("create", "--name", t.Name())
		require.NoError(t, cmd.Run())
	}

	cmd := apiKeyCommand("invalidate", "--name", t.Name(), "--json")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(out, &m)
	require.NoError(t, err)
	assert.Len(t, m["invalidated_api_keys"], 2)
}
