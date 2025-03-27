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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func New(kibanaURL, apiKey string, username, password string) (*Client, error) {
	if kibanaURL == "" {
		return nil, fmt.Errorf("kbclient.New kibanaURL must not be empty")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("kbclient.New apiKey must not be empty")
	}

	return &Client{
		url:                 kibanaURL,
		apiKey:              apiKey,
		superUsername:       username,
		superPassword:       password,
		SupportedAPIVersion: "2023-10-31",
	}, nil
}

// Client is a wrapped HTTP Client with custom Kibana related methods.
type Client struct {
	http.Client
	// url is the Kibana URL where requests will be directed to.
	url string
	// apiKey should be an Elasticsearch API key with appropriate privileges.
	apiKey string

	// superUsername should be an Elasticsearch superuser username.
	superUsername string
	// superPassword should be an Elasticsearch superuser password.
	superPassword string

	SupportedAPIVersion string
}

// prepareRequest creates a http.Request with required headers for interacting with Kibana.
func (c *Client) prepareRequest(method, path string, body any, super bool) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal body: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.url, path)
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %w", err)
	}

	req.Header.Add("kbn-xsrf", "true")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Elastic-Api-Version", c.SupportedAPIVersion)
	if super {
		userPass := fmt.Sprintf("%s:%s", c.superUsername, c.superPassword)
		basicAuth := base64.StdEncoding.EncodeToString([]byte(userPass))
		req.Header.Add("Authorization", fmt.Sprintf("Basic %s", basicAuth))
	} else {
		req.Header.Add("Authorization", fmt.Sprintf("ApiKey %s", c.apiKey))
	}

	return req, nil
}

// sendRequest sends a http.Request with the provided method, path and body to Kibana
// and returns the response body bytes if the status code is 200.
// It will also handle non-200 status codes accordingly, if the handler is defined.
func (c *Client) sendRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
	super bool,
	handleRespError func(statusCode int, body []byte) error,
) ([]byte, error) {
	req, err := c.prepareRequest(method, path, body, super)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare request %s %s: %w", method, path, err)
	}

	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body %s %s: %w", method, path, err)
	}

	if resp.StatusCode != 200 {
		if handleRespError != nil {
			if err = handleRespError(resp.StatusCode, b); err != nil {
				return nil, err
			}
		}
		if len(b) > 0 {
			return nil, fmt.Errorf("request failed with status code %d, body: %v", resp.StatusCode, b)
		}
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return b, nil
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
func (c *Client) GetPackagePolicyByID(ctx context.Context, policyID string) (PackagePolicy, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	handleRespError := func(statusCode int, _ []byte) error {
		if statusCode == 404 {
			return &ElasticAgentPolicyNotFoundError{Name: policyID}
		}
		return nil
	}

	b, err := c.sendRequest(ctx, http.MethodGet, path, nil, false, handleRespError)
	if err != nil {
		return PackagePolicy{}, err
	}

	var policyResp getPackagePolicyResponse
	if err = json.Unmarshal(b, &policyResp); err != nil {
		return PackagePolicy{}, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return policyResp.Item, nil
}

type UpdatePackagePolicyRequest struct {
	PackagePolicy
	Force bool `json:"force"`
}

// UpdatePackagePolicyByID performs a Package Policy update in Fleet through the Fleet Kibana APIs.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-update-package-policy
func (c *Client) UpdatePackagePolicyByID(ctx context.Context, policyID string, request UpdatePackagePolicyRequest) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	_, err := c.sendRequest(ctx, http.MethodPut, path, request, false, nil)
	return err
}

type enableIntegrationsResponse struct {
	CloudApmPackagePolicy struct {
		Enabled bool `json:"enabled"`
	} `json:"cloudApmPackagePolicy"`
}

// EnableIntegrationsServer enables integrations server to add combined APM and Fleet Server to the deployment.
// https://www.elastic.co/guide/en/cloud/current/ec-manage-integrations-server.html
func (c *Client) EnableIntegrationsServer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// This is an internal API that is not publicly documented.
	// https://github.com/elastic/kibana/blob/12aa3fc/x-pack/solutions/observability/plugins/apm/server/routes/fleet/route.ts#L146
	path := "/internal/apm/fleet/cloud_apm_package_policy"
	b, err := c.sendRequest(ctx, http.MethodPost, path, nil, true, nil)
	if err != nil {
		return err
	}

	var resp enableIntegrationsResponse
	if err = json.Unmarshal(b, &resp); err != nil {
		return fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	if !resp.CloudApmPackagePolicy.Enabled {
		return errors.New("failed to enable integrations server")
	}

	return nil
}
