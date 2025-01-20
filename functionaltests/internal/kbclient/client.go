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

package kbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func New(kibanaURL, apikey string) *Client {
	return &Client{
		url:    kibanaURL,
		apikey: apikey,
	}
}

type Client struct {
	http.Client
	url    string
	apikey string
}

type AgentPoliciesResponse struct {
	Items []struct {
		ID              string
		PackagePolicies []struct {
			ID string
		} `json:"package_policies"`
	}
}

// AgentPolicies retrieves available Agent policies.
func (c *Client) AgentPolicies(ctx context.Context) (AgentPoliciesResponse, error) {
	req, err := prepareRequest(c, http.MethodGet, "api/fleet/agent_policies?full=true", nil)
	if err != nil {
		return AgentPoliciesResponse{}, fmt.Errorf("cannot prepare AgentPolicies request: %w", err)
	}

	resp, err := c.Do(req.WithContext(ctx))
	if err != nil {
		return AgentPoliciesResponse{}, fmt.Errorf("cannot retrieve agent policies: %w", err)
	}

	v, err := io.ReadAll(resp.Body)
	if err != nil {
		return AgentPoliciesResponse{}, fmt.Errorf("cannot read response body: %w", err)
	}

	var res AgentPoliciesResponse
	err = json.Unmarshal(v, &res)
	if err != nil {
		return AgentPoliciesResponse{}, fmt.Errorf("cannot unmarshal to AgentPoliciesResponse: %w", err)
	}

	return res, nil
}

func prepareRequest(c *Client, method, path string, body any) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return &http.Request{}, fmt.Errorf("cannot marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/%s", c.url, path)
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return &http.Request{}, fmt.Errorf("cannot create http request to %s: %w", url, err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", c.apikey))
	req.Header.Add("kbn-xsrf", "true")
	req.Header.Add("Content-Type", "application/json")

	return req, nil
}
