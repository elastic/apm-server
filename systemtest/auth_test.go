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
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestAuth(t *testing.T) {
	systemtest.InvalidateAPIKeys(t)
	defer systemtest.InvalidateAPIKeys(t)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	secretToken := strconv.Itoa(rng.Int())

	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.AgentAuth.SecretToken = secretToken
	srv.Config.AgentAuth.APIKey = &apmservertest.APIKeyAuthConfig{Enabled: true}
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	apiKey := createAPIKey(t, t.Name()+":all")
	apiKeySourcemap := createAPIKey(t, t.Name()+":sourcemap", "--sourcemap")
	apiKeyIngest := createAPIKey(t, t.Name()+":ingest", "--ingest")
	apiKeyAgentConfig := createAPIKey(t, t.Name()+":agentconfig", "--agent-config")

	runWithMethods := func(t *testing.T, name string, f func(t *testing.T, apiKey string, headers http.Header)) {
		t.Run(name, func(t *testing.T) {
			t.Run("anonymous", func(t *testing.T) { f(t, "", nil) })
			t.Run("secret_token", func(t *testing.T) {
				f(t, "", http.Header{"Authorization": []string{"Bearer " + secretToken}})
			})
			t.Run("api_key", func(t *testing.T) {
				f(t, "all", http.Header{"Authorization": []string{"ApiKey " + apiKey}})
				f(t, "sourcemap", http.Header{"Authorization": []string{"ApiKey " + apiKeySourcemap}})
				f(t, "ingest", http.Header{"Authorization": []string{"ApiKey " + apiKeyIngest}})
				f(t, "agentconfig", http.Header{"Authorization": []string{"ApiKey " + apiKeyAgentConfig}})
			})
		})
	}

	runWithMethods(t, "root", func(t *testing.T, apiKey string, headers http.Header) {
		req, _ := http.NewRequest("GET", srv.URL, nil)
		copyHeaders(req.Header, headers)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		if len(headers) == 0 {
			assert.Empty(t, body)
		} else {
			assert.NotEmpty(t, body)
		}
	})

	eventsPayload, err := ioutil.ReadFile("../testdata/intake-v2/transactions.ndjson")
	require.NoError(t, err)
	runWithMethods(t, "ingest", func(t *testing.T, apiKey string, headers http.Header) {
		req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/events", bytes.NewReader(eventsPayload))
		req.Header.Set("Content-Type", "application/x-ndjson")
		copyHeaders(req.Header, headers)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		if len(headers) == 0 || apiKey == "sourcemap" || apiKey == "agentconfig" {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		} else {
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		}
	})

	runWithMethods(t, "sourcemap", func(t *testing.T, apiKey string, headers http.Header) {
		req := newUploadSourcemapRequest(t, srv, "../testdata/sourcemap/bundle.js.map",
			"http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map", // bundle filepath
			"apm-agent-js", // service name
			"1.0.1",        // service version
		)
		copyHeaders(req.Header, headers)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		if len(headers) == 0 || apiKey == "ingest" || apiKey == "agentconfig" {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		} else {
			assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		}
	})

	// Create agent config to test the anonymous and authenticated responses.
	settings := map[string]string{"transaction_sample_rate": "0.1", "sanitize_field_names": "foo,bar,baz"}
	systemtest.CreateAgentConfig(t, "systemtest_service", "", "", settings)
	completeSettings := `{"sanitize_field_names":"foo,bar,baz","transaction_sample_rate":"0.1"}`
	anonymousSettings := `{"transaction_sample_rate":"0.1"}`
	runWithMethods(t, "agentconfig", func(t *testing.T, apiKey string, headers http.Header) {
		req, _ := http.NewRequest("GET", srv.URL+"/config/v1/agents", nil)
		copyHeaders(req.Header, headers)
		req.Header.Add("Content-Type", "application/json")
		req.URL.RawQuery = url.Values{"service.name": []string{"systemtest_service"}}.Encode()
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		if apiKey == "ingest" || apiKey == "sourcemap" {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		} else {
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, _ := ioutil.ReadAll(resp.Body)
			if len(headers) == 0 {
				// Anonymous auth succeeds because RUM is enabled, which
				// auto enables anonymous auth. However, only a subset of
				// the config is returned.
				assert.Equal(t, anonymousSettings, strings.TrimSpace(string(body)))
			} else {
				assert.Equal(t, completeSettings, strings.TrimSpace(string(body)))
			}
		}
	})
}

func copyHeaders(to, from http.Header) {
	for k, values := range from {
		for _, v := range values {
			to.Add(k, v)
		}
	}
}

func createAPIKey(t *testing.T, name string, args ...string) string {
	args = append([]string{"--name", name, "--json"}, args...)
	cmd := apiKeyCommand("create", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	attrs := decodeJSONMap(t, bytes.NewReader(out))
	return attrs["credentials"].(string)
}
