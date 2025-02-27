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

	"github.com/itchyny/gojq"
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

// GetPackagePolicyByID retrieves the Package Policy specified by policyID.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-get-package-policy
func (c *Client) GetPackagePolicyByID(policyID string) ([]byte, error) {
	var b []byte

	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	req, err := prepareRequest(c, http.MethodGet, path, nil)
	if err != nil {
		return b, fmt.Errorf("cannot prepare request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return b, fmt.Errorf("cannot perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return b, fmt.Errorf("cannot read response body: %w", err)
	}

	if resp.StatusCode == 404 {
		return b, &ElasticAgentPolicyNotFoundError{Name: policyID}
	}

	if resp.StatusCode > 200 {
		return b, fmt.Errorf("%s request failed with status code %d", path, resp.StatusCode)
	}

	return b, nil
}

// UpdatePackagePolicyByID performs a Package Policy update in Fleet through the Fleet Kibana APIs.
// Updating the elastic-cloud-apm package policy, even without
// modifying any field will trigger final aggregations.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-update-package-policy
func (c *Client) UpdatePackagePolicyByID(policyID string, data any) error {
	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	req, err := prepareRequest(c, http.MethodPut, path, data)
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

// TouchPackagePolicyByID sends a PUT to the UpdatePackagePolicyByID Fleet API endpoint to
// trigger a reload of configuration in beats linked to the policy.
// For example useful to trigger a flush for APM Server metrics.
func (c *Client) TouchPackagePolicyByID(policyID string) error {
	eca, err := c.GetPackagePolicyByID(policyID)
	if err != nil {
		return fmt.Errorf("cannot get package policy elastic-cloud-apm: %w", err)
	}

	var f interface{}
	if err := json.Unmarshal(eca, &f); err != nil {
		return fmt.Errorf("cannot unmarshal package policy: %w", err)
	}

	// these are the minimum modifications required for sending the
	// policy back to Fleet. Any less and you'll get a validation error.
	v, err := jq(".item", f)
	if err != nil {
		return fmt.Errorf("cannot extract data from .item in policy JSON: %w", err)
	}
	v, err = jq("del(.id) | del(.elasticsearch) | del(.inputs[].compiled_input) | del(.revision) | del(.created_at) | del(.created_by) | del(.updated_at) | del(.updated_by)", v)
	if err != nil {
		return fmt.Errorf("cannot clean up data in policy JSON: %w", err)
	}

	// We don't want to modify the policy, only to send a PUT with a valid payload
	// TODO: with slight modifications this can update relevant apm policy vars
	// see https://github.com/elastic/apm-server/blob/1261584b25b5279b1fcc1e6707dd07d31e708f38/testing/infra/terraform/modules/ec_deployment/scripts/enable_features.tftpl#L11-L10
	err = c.UpdatePackagePolicyByID(policyID, v)
	if err != nil {
		return fmt.Errorf("cannot update package policy: %w", err)
	}

	return nil
}

// jq runs jq instruction q on data. It expects a single object to be
// returned from the expression, as it does not iterate on the result
// set.
func jq(q string, data any) (any, error) {
	query, err := gojq.Parse(q)
	if err != nil {
		return nil, fmt.Errorf("cannot parse jq expression: %w", err)
	}
	iter := query.Run(data)
	v, ok := iter.Next()
	if !ok {
		return v, fmt.Errorf("cannot iterate on jq query result")
	}
	if err, ok := v.(error); ok {
		return nil, fmt.Errorf("iterator returned an error: %w", err)
	}
	return v, nil
}
