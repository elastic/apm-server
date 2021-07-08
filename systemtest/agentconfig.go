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

package systemtest

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

// CreateAgentConfig creates or updates agent central config via Kibana.
func CreateAgentConfig(t testing.TB, serviceName, serviceEnvironment, agentName string, settings map[string]string) {
	kibanaConfig := apmservertest.DefaultConfig().Kibana
	kibanaURL, err := url.Parse(kibanaConfig.Host)
	if err != nil {
		t.Fatal(err)
	}
	kibanaURL.User = url.UserPassword(kibanaConfig.Username, kibanaConfig.Password)
	kibanaURL.Path = "/api/apm/settings/agent-configuration"
	kibanaURL.RawQuery = "overwrite=true"

	var params struct {
		AgentName string `json:"agent_name,omitempty"`
		Service   struct {
			Name        string `json:"name"`
			Environment string `json:"environment,omitempty"`
		} `json:"service"`
		Settings map[string]string `json:"settings"`
	}
	params.Service.Name = serviceName
	params.Service.Environment = serviceEnvironment
	params.AgentName = agentName
	params.Settings = settings

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(params); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("PUT", kibanaURL.String(), &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("kbn-xsrf", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("failed to create agent config: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
}
