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

// Package kbclient implements a Kibana HTTP API client to perform operation needed by functional tests.
package kbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func New(kibanaURL, apikey string) (*Client, error) {
	if kibanaURL == "" {
		return nil, fmt.Errorf("kbclient.New kibanaURL must not be empty")
	}
	if apikey == "" {
		return nil, fmt.Errorf("kbclient.New apikey must not be empty")
	}

	return &Client{
		url:                 kibanaURL,
		apikey:              apikey,
		SupportedAPIVersion: "2023-10-31",
	}, nil
}

// Client is a wrapped HTTP Client with custom Kibana related methods.
type Client struct {
	http.Client
	// url is the Kibana URL where requests will be directed to.
	url string
	// apikey should be an Elasticsearch API key with appropriate privileges.
	apikey string

	SupportedAPIVersion string
}

// prepareRequest creates a http.Request with required headers for interacting with Kibana.
func prepareRequest(c *Client, method, path string, body any) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal body: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.url, path)
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", c.apikey))
	req.Header.Add("kbn-xsrf", "true")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Elastic-Api-Version", c.SupportedAPIVersion)

	return req, nil
}

type ElasticAgentPolicyNotFoundError struct {
	Name string
}

func (e ElasticAgentPolicyNotFoundError) Error() string {
	return fmt.Sprintf("ElasticAgentPolicy named %s was not found", e.Name)
}

type PackagePolicy struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Package     PackagePolicyPkg `json:"package"`
	PolicyID    string           `json:"policy_id"`
	PolicyIDs   []string         `json:"policy_ids"`
}

type PackagePolicyPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type getPackagePolicyResponse struct {
	Item PackagePolicy `json:"item"`
}

// GetPackagePolicyByID retrieves the Package Policy specified by policyID.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-get-package-policy
func (c *Client) GetPackagePolicyByID(policyID string) (PackagePolicy, error) {
	var empty PackagePolicy

	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	req, err := prepareRequest(c, http.MethodGet, path, nil)
	if err != nil {
		return empty, fmt.Errorf("cannot prepare request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return empty, fmt.Errorf("cannot perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return empty, &ElasticAgentPolicyNotFoundError{Name: policyID}
	}

	if resp.StatusCode > 200 {
		return empty, fmt.Errorf("%s request failed with status code %d", path, resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, fmt.Errorf("cannot read response body: %w", err)
	}

	var policyResp getPackagePolicyResponse
	if err = json.Unmarshal(b, &policyResp); err != nil {
		return empty, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return policyResp.Item, nil
}

type UpdatePackagePolicyRequest struct {
	PackagePolicy
	Force bool `json:"force"`
}

// UpdatePackagePolicyByID performs a Package Policy update in Fleet through the Fleet Kibana APIs.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-update-package-policy
func (c *Client) UpdatePackagePolicyByID(policyID string, request UpdatePackagePolicyRequest) error {
	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	req, err := prepareRequest(c, http.MethodPut, path, request)
	if err != nil {
		return fmt.Errorf("cannot prepare request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("cannot perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("cannot read response body: %w", err)
		}

		return fmt.Errorf("request returned status code %d with body: %s", resp.StatusCode, b)
	}

	return nil
}
