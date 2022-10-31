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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func apiKeyCommand(subcommand string, args ...string) *apmservertest.ServerCmd {
	cfg := apmservertest.DefaultConfig()
	return apiKeyCommandConfig(cfg, subcommand, args...)
}

func apiKeyCommandConfig(cfg apmservertest.Config, subcommand string, args ...string) *apmservertest.ServerCmd {
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
	return apmservertest.ServerCommand(context.Background(), "apikey", args...)
}

func TestAPIKeyCreate(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	cmd := apiKeyCommand("create", "--name", t.Name(), "--json")
	out, err := cmd.Output()
	require.NoError(t, err)

	attrs := decodeJSONMap(t, bytes.NewReader(out))
	assert.Equal(t, t.Name(), attrs["name"])
	assert.Contains(t, attrs, "id")
	assert.Contains(t, attrs, "api_key")
	assert.Contains(t, attrs, "credentials")

	es := systemtest.NewElasticsearchClientWithAPIKey(attrs["credentials"].(string))
	assertAuthenticateSucceeds(t, es)

	// Check that the API Key has expected metadata.
	type apiKey struct {
		ID       string                 `json:"id"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	var resp struct {
		APIKeys []apiKey `json:"api_keys"`
	}
	_, err = systemtest.Elasticsearch.Do(context.Background(), &esapi.SecurityGetAPIKeyRequest{
		ID: attrs["id"].(string),
	}, &resp)
	require.NoError(t, err)
	require.Len(t, resp.APIKeys, 1)
	assert.Equal(t, map[string]interface{}{"application": "apm"}, resp.APIKeys[0].Metadata)
}

func TestAPIKeyCreateExpiration(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	cmd := apiKeyCommand("create", "--name", t.Name(), "--json", "--expiration=1d")
	out, err := cmd.Output()
	require.NoError(t, err)

	attrs := decodeJSONMap(t, bytes.NewReader(out))
	assert.Contains(t, attrs, "expiration")
}

func TestAPIKeyCreateInvalidUser(t *testing.T) {
	// heartbeat_user lacks cluster privileges, and cannot create keys
	// beats_user has cluster privileges, but not APM application privileges
	for _, username := range []string{"heartbeat_user", "beats_user"} {
		cfg := apmservertest.DefaultConfig()
		cfg.Output.Elasticsearch.Username = username
		cfg.Output.Elasticsearch.Password = "changeme"

		cmd := apiKeyCommandConfig(cfg, "create", "--name", t.Name(), "--json")
		out, err := cmd.Output()
		require.Error(t, err)
		attrs := decodeJSONMap(t, bytes.NewReader(out))
		assert.Regexp(t, username+` is missing the following requested privilege\(s\): .*`, attrs["error"])
	}
}

func TestAPIKeyInvalidateName(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	var clients []*estest.Client
	for i := 0; i < 2; i++ {
		cmd := apiKeyCommand("create", "--name", t.Name(), "--json")
		out, err := cmd.Output()
		require.NoError(t, err)

		attrs := decodeJSONMap(t, bytes.NewReader(out))
		es := systemtest.NewElasticsearchClientWithAPIKey(attrs["credentials"].(string))
		assertAuthenticateSucceeds(t, es)
		clients = append(clients, es)
	}

	cmd := apiKeyCommand("invalidate", "--name", t.Name(), "--json")
	out, err := cmd.Output()
	require.NoError(t, err)

	result := decodeJSONMap(t, bytes.NewReader(out))
	assert.Len(t, result["invalidated_api_keys"], 2)
	assert.Equal(t, float64(0), result["error_count"])

	for _, es := range clients {
		assertAuthenticateFails(t, es)
	}
}

func TestAPIKeyInvalidateID(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	cmd := apiKeyCommand("create", "--json")
	out, err := cmd.Output()
	require.NoError(t, err)
	attrs := decodeJSONMap(t, bytes.NewReader(out))

	es := systemtest.NewElasticsearchClientWithAPIKey(attrs["credentials"].(string))
	assertAuthenticateSucceeds(t, es)

	// NOTE(axw) it is important to use "--id=<id>" rather than "--id" <id>,
	// as API keys may begin with a hyphen and be interpreted as flags.
	cmd = apiKeyCommand("invalidate", "--json", "--id="+attrs["id"].(string))
	out, err = cmd.Output()
	require.NoError(t, err)
	result := decodeJSONMap(t, bytes.NewReader(out))

	assert.Equal(t, []interface{}{attrs["id"]}, result["invalidated_api_keys"])
	assert.Equal(t, float64(0), result["error_count"])
	assertAuthenticateFails(t, es)
}

func TestAPIKeyVerify(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	cmd := apiKeyCommand("create", "--name", t.Name(), "--json", "--ingest", "--agent-config")
	out, err := cmd.Output()
	require.NoError(t, err)
	attrs := decodeJSONMap(t, bytes.NewReader(out))
	credentials := attrs["credentials"].(string)

	cmd = apiKeyCommand("verify", "--json", "--credentials="+credentials)
	out, err = cmd.Output()
	require.NoError(t, err)
	attrs = decodeJSONMap(t, bytes.NewReader(out))
	assert.Equal(t, map[string]interface{}{
		"event:write":       true,
		"config_agent:read": true,
		"sourcemap:write":   false,
	}, attrs)

	cmd = apiKeyCommand("verify", "--json", "--credentials="+credentials, "--ingest")
	out, err = cmd.Output()
	require.NoError(t, err)
	attrs = decodeJSONMap(t, bytes.NewReader(out))
	assert.Equal(t, map[string]interface{}{"event:write": true}, attrs)
}

func TestAPIKeyInfo(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	var ids []string
	for i := 0; i < 2; i++ {
		cmd := apiKeyCommand("create", "--name", t.Name(), "--json", "--ingest", "--agent-config")
		out, err := cmd.Output()
		require.NoError(t, err)
		attrs := decodeJSONMap(t, bytes.NewReader(out))
		ids = append(ids, attrs["id"].(string))
	}

	type apiKey struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var result struct {
		APIKeys []apiKey `json:"api_keys"`
	}

	cmd := apiKeyCommand("info", "--json", "--id="+ids[0])
	out, err := cmd.Output()
	require.NoError(t, err)
	err = json.Unmarshal(out, &result)
	require.NoError(t, err)
	assert.Equal(t, []apiKey{{
		ID:   ids[0],
		Name: t.Name(),
	}}, result.APIKeys)

	result.APIKeys = nil
	cmd = apiKeyCommand("info", "--json", "--name="+t.Name())
	out, err = cmd.Output()
	require.NoError(t, err)
	err = json.Unmarshal(out, &result)
	require.NoError(t, err)
	// Should be at least 2, possibly more; Elasticsearch may
	// hold invalidated keys from previous test runs.
	assert.GreaterOrEqual(t, len(result.APIKeys), 2)
}

func assertAuthenticateSucceeds(t testing.TB, es *estest.Client) *esapi.Response {
	t.Helper()
	resp, err := es.Security.Authenticate()
	require.NoError(t, err)
	assert.False(t, resp.IsError())
	return resp
}

func assertAuthenticateFails(t testing.TB, es *estest.Client) *esapi.Response {
	t.Helper()
	resp, err := es.Security.Authenticate()
	require.NoError(t, err)
	assert.True(t, resp.IsError())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	return resp
}

func decodeJSONMap(t *testing.T, r io.Reader) map[string]interface{} {
	var m map[string]interface{}
	err := json.NewDecoder(r).Decode(&m)
	require.NoError(t, err)
	return m
}
