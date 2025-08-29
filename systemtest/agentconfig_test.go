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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestAgentConfig(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	systemtest.GeoIpLazyDownload(t)

	serviceName := "systemtest_service"
	serviceEnvironment := "testing"
	systemtest.DeleteAgentConfig(t, serviceName, "")
	systemtest.DeleteAgentConfig(t, serviceName, serviceEnvironment)

	// Run apm-server standalone, exercising the Elasticsearch agent config implementation.
	srvElasticsearch := apmservertest.NewUnstartedServerTB(t)
	srvElasticsearch.Config.AgentConfig = &apmservertest.AgentConfig{
		CacheExpiration:       time.Second,
		ElasticsearchUsername: "admin",
		ElasticsearchPassword: "changeme",
	}
	err := srvElasticsearch.Start()
	require.NoError(t, err)

	// Run apm-server standalone, exercising the Kibana agent config implementation.
	srvKibana := apmservertest.NewUnstartedServerTB(t)
	srvKibana.Config.AgentConfig = &apmservertest.AgentConfig{CacheExpiration: time.Second}
	err = srvKibana.Start()
	require.NoError(t, err)

	serverURLs := []string{srvElasticsearch.URL, srvKibana.URL}

	expectChange := func(serverURL string, etag string) (map[string]string, *http.Response) {
		t.Helper()
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		interval := 100 * time.Millisecond
		for {
			settings, resp, _ := queryAgentConfig(t, serverURL, serviceName, serviceEnvironment, etag)
			if resp.StatusCode == http.StatusOK {
				return settings, resp
			}
			select {
			case <-timer.C:
				t.Fatal("timed out waiting for agent config change")
			case <-time.After(interval):
			}
		}
	}

	// No agent config matching service name/environment initially.
	for _, url := range serverURLs {
		settings, resp := expectChange(url, "")
		assert.Empty(t, settings)
		etag := resp.Header.Get("Etag")
		assert.Equal(t, `"-"`, etag)
		_, resp, _ = queryAgentConfig(t, url, serviceName, serviceEnvironment, etag)
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	}

	// Create an agent config entry matching the service name, and any environment.
	configured := map[string]string{"transaction_sample_rate": "0.1", "sanitize_field_names": "foo,bar,baz"}
	systemtest.CreateAgentConfig(t, "systemtest_service", "", "", configured)
	var etag1 string
	for i, url := range serverURLs {
		settings, resp := expectChange(url, "-")
		assert.Equal(t, configured, settings)
		etag := resp.Header.Get("Etag")
		assert.NotEqual(t, `"-"`, etag)
		_, resp, _ = queryAgentConfig(t, url, serviceName, serviceEnvironment, etag)
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
		if i == 0 {
			etag1 = etag
		} else {
			assert.Equal(t, etag1, etag)
		}
	}

	// Create a more specific agent config entry with both service name and environment
	// matching the query. This should now take precedence.
	configured2 := map[string]string{"transaction_sample_rate": "0.2"}
	systemtest.CreateAgentConfig(t, "systemtest_service", "testing", "", configured2)
	var etag2 string
	for i, url := range serverURLs {
		settings, resp := expectChange(url, etag1)
		assert.Equal(t, configured2, settings)
		etag := resp.Header.Get("Etag")
		assert.NotEqual(t, etag1, etag)
		if i == 0 {
			etag2 = etag
		} else {
			assert.Equal(t, etag2, etag)
		}
	}

	// Wait for an "agent_config" metricset to be reported, which should contain the
	// etag of the configuration as a label. We should only receive one metricset per
	// server, as we only produce these documents when the etag matches.
	//
	// Known issue: APM integration does not produce documents because it does not have
	// enough time to flush before reload
	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.internal-*", espoll.TermQuery{
		Field: "metricset.name",
		Value: "agent_config",
	}, espoll.WithTimeout(time.Minute))
	require.Len(t, result.Hits.Hits, 2)
	etag := gjson.GetBytes(result.Hits.Hits[0].RawSource, "labels.etag")
	assert.Equal(t, etag1, strconv.Quote(etag.String()))
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits, "@timestamp", "labels.etag")
}

func queryAgentConfig(t testing.TB, serverURL, serviceName, serviceEnvironment, etag string) (map[string]string, *http.Response, map[string]interface{}) {
	query := make(url.Values)
	query.Set("service.name", serviceName)
	if serviceEnvironment != "" {
		query.Set("service.environment", serviceEnvironment)
	}
	url, _ := url.Parse(serverURL + "/config/v1/agents")
	url.RawQuery = query.Encode()

	req, _ := http.NewRequest("GET", url.String(), nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	c := http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)

	maxRetries := 10
	var retries int
	for err != nil && retries < maxRetries {
		retries++
		t.Logf(`apm-server returned err="%v" on read, retry %d/%d...`, err, retries, maxRetries)
		<-time.After(500 * time.Millisecond)
		resp, err = c.Do(req)
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	errorObj := make(map[string]interface{})
	attrs := make(map[string]string)
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&attrs)
		require.NoError(t, err)
	case http.StatusNotModified:
	case http.StatusForbidden:
		err = json.NewDecoder(resp.Body).Decode(&errorObj)
		require.NoError(t, err)
	case http.StatusServiceUnavailable:
		err = json.NewDecoder(resp.Body).Decode(&errorObj)
		require.NoError(t, err)
	default:
		t.Fatalf("unexpected status %q", resp.Status)
	}
	return attrs, resp, errorObj
}

func TestAgentConfigForbiddenOnInvalidConfig(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	serviceName := "systemtest_service"
	serviceEnvironment := "testing"

	// Unauthorized username and password.
	// No fallback because apm-server.agent.config.elasticsearch.username
	// and apm-server.agent.config.elasticsearch.password are explicitly configured.
	srvInvalidUsernamePasswordSetup := func(t *testing.T) *apmservertest.Server {
		srv := apmservertest.NewUnstartedServerTB(t)
		srv.Config.AgentConfig = &apmservertest.AgentConfig{
			CacheExpiration:       time.Second,
			ElasticsearchUsername: "bad_username",
			ElasticsearchPassword: "bad_password",
		}
		srv.Config.Kibana = nil
		err := srv.Start()
		require.NoError(t, err)
		return srv
	}

	// Unauthorized API key.
	// No fallback because apm-server.agent.config.elasticsearch.api_key is explicitly configured.
	srvInvalidAPIKeySetup := func(t *testing.T) *apmservertest.Server {
		srv := apmservertest.NewUnstartedServerTB(t)
		srv.Config.AgentConfig = &apmservertest.AgentConfig{
			CacheExpiration:     time.Second,
			ElasticsearchAPIKey: "bad_api_key",
		}
		srv.Config.Kibana = nil
		err := srv.Start()
		require.NoError(t, err)
		return srv
	}

	// Username and password with insufficient privileges.
	// No fallback because apm-server.agent.config.elasticsearch.username
	// and apm-server.agent.config.elasticsearch.password are explicitly configured.
	srvInsufficientPrivilegesUsernamePasswordSetup := func(t *testing.T) *apmservertest.Server {
		srv := apmservertest.NewUnstartedServerTB(t)
		srv.Config.AgentConfig = &apmservertest.AgentConfig{
			CacheExpiration:       time.Second,
			ElasticsearchUsername: "apm_server_user",
			ElasticsearchPassword: "changeme",
		}
		srv.Config.Kibana = nil
		err := srv.Start()
		require.NoError(t, err)
		return srv
	}

	// API key with insufficient privileges.
	// No fallback because apm-server.agent.config.elasticsearch.api_key is explicitly configured.
	srvInsufficientPrivilegesAPIKeySetup := func(t *testing.T) *apmservertest.Server {
		apiKeyName := t.Name()
		systemtest.InvalidateAPIKeyByName(t, apiKeyName)
		t.Cleanup(func() {
			systemtest.InvalidateAPIKeyByName(t, apiKeyName)
		})
		// Create an API Key without agent config read privileges
		apiKeyBase64 := systemtest.CreateAPIKey(t, apiKeyName, []string{"sourcemap:write"})
		apiKeyBytes, err := base64.StdEncoding.DecodeString(apiKeyBase64)
		require.NoError(t, err)
		srv := apmservertest.NewUnstartedServerTB(t)
		srv.Config.AgentConfig = &apmservertest.AgentConfig{
			CacheExpiration:     time.Second,
			ElasticsearchAPIKey: string(apiKeyBytes),
		}
		srv.Config.Kibana = nil
		err = srv.Start()
		require.NoError(t, err)
		return srv
	}

	tests := []struct {
		name     string
		srvSetup func(*testing.T) *apmservertest.Server
	}{
		{
			name:     "invalid_username_password",
			srvSetup: srvInvalidUsernamePasswordSetup,
		},
		{
			name:     "invalid_api_key",
			srvSetup: srvInvalidAPIKeySetup,
		},
		{
			name:     "insufficient_privileges_username_password",
			srvSetup: srvInsufficientPrivilegesUsernamePasswordSetup,
		},
		{
			name:     "insufficient_privileges_api_key",
			srvSetup: srvInsufficientPrivilegesAPIKeySetup,
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := tc.srvSetup(t)
			url := srv.URL

			expectedErrorMsg := "Your Elasticsearch configuration does not support agent config queries. Check your configurations at `output.elasticsearch` or `apm-server.agent.config.elasticsearch`."

			var resp *http.Response
			var errorObj map[string]interface{}
			maxRetries := 20
			for i := 0; i < maxRetries; i++ {
				_, resp, errorObj = queryAgentConfig(t, url, serviceName, serviceEnvironment, "")
				if resp.StatusCode == http.StatusForbidden {
					break
				}
				<-time.After(500 * time.Millisecond)
			}

			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			assert.Equal(t, expectedErrorMsg, errorObj["error"].(string))
		})
	}

}
