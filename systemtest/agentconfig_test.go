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
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestAgentConfig(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	serviceName := "systemtest_service"
	serviceEnvironment := "testing"
	systemtest.DeleteAgentConfig(t, serviceName, "")
	systemtest.DeleteAgentConfig(t, serviceName, serviceEnvironment)

	// Run apm-server standalone, exercising the Kibana agent config implementation.
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.KibanaAgentConfig = &apmservertest.KibanaAgentConfig{CacheExpiration: time.Second}
	err := srv.Start()
	require.NoError(t, err)

	// Run apm-server under Fleet, exercising the Fleet agent config implementation.
	apmIntegration := newAPMIntegration(t, map[string]interface{}{})
	serverURLs := []string{srv.URL, apmIntegration.URL}

	expectChange := func(serverURL string, etag string) (map[string]string, *http.Response) {
		t.Helper()
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		interval := 100 * time.Millisecond
		for {
			settings, resp := queryAgentConfig(t, serverURL, serviceName, serviceEnvironment, etag)
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
	etags := make(map[string]string)
	for _, url := range serverURLs {
		settings, resp := expectChange(url, "")
		assert.Empty(t, settings)
		etag := resp.Header.Get("Etag")
		assert.Equal(t, `"-"`, etag)
		etags[url] = etag
		_, resp = queryAgentConfig(t, url, serviceName, serviceEnvironment, etag)
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	}

	// Create an agent config entry matching the service name, and any environment.
	configured := map[string]string{"transaction_sample_rate": "0.1", "sanitize_field_names": "foo,bar,baz"}
	systemtest.CreateAgentConfig(t, "systemtest_service", "", "", configured)
	for _, url := range serverURLs {
		settings, resp := expectChange(url, etags[url])
		assert.Equal(t, configured, settings)
		etag := resp.Header.Get("Etag")
		assert.NotEqual(t, `"-"`, etag)
		etags[url] = etag
		_, resp = queryAgentConfig(t, url, serviceName, serviceEnvironment, etag)
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	}

	// Create a more specific agent config entry with both service name and environment
	// matching the query. This should now take precedence.
	configured2 := map[string]string{"transaction_sample_rate": "0.2"}
	systemtest.CreateAgentConfig(t, "systemtest_service", "testing", "", configured2)
	for _, url := range serverURLs {
		settings, resp := expectChange(url, etags[url])
		assert.Equal(t, configured2, settings)
		assert.NotEqual(t, etags[url], resp.Header.Get("Etag"))
	}
}

func queryAgentConfig(t testing.TB, serverURL, serviceName, serviceEnvironment, etag string) (map[string]string, *http.Response) {
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
	resp, err := http.DefaultClient.Do(req)

	maxRetries := 10
	var retries int
	for errors.Is(err, io.EOF) && retries < maxRetries {
		retries++
		t.Logf("apm-server returned EOF on read, retry %d/%d...", retries, maxRetries)
		<-time.After(500 * time.Millisecond)
		resp, err = http.DefaultClient.Do(req)
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	attrs := make(map[string]string)
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&attrs)
		require.NoError(t, err)
	case http.StatusNotModified:
	default:
		t.Fatalf("unexpected status %q", resp.Status)
	}
	return attrs, resp
}
