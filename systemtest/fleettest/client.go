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

package fleettest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Client provides methods for interacting with the Fleet API.
type Client struct {
	fleetURL string
}

// NewClient returns a new Client for interacting with the Fleet API,
// using the given Kibana URL.
func NewClient(kibanaURL string) *Client {
	return &Client{fleetURL: kibanaURL + "/api/fleet"}
}

// Setup invokes the Fleet Setup API, returning an error if it fails.
func (c *Client) Setup() error {
	for _, path := range []string{"/setup", "/agents/setup"} {
		req := c.newFleetRequest("POST", path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("request failed (%s): %s", resp.Status, body)
		}
	}
	return nil
}

// Agents returns the list of enrolled agents.
func (c *Client) Agents() ([]Agent, error) {
	resp, err := http.Get(c.fleetURL + "/agents")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	var result struct {
		List []Agent `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.List, nil
}

// BulkUnenrollAgents bulk-unenrolls agents.
func (c *Client) BulkUnenrollAgents(force bool, agentIDs ...string) error {
	var body bytes.Buffer
	type bulkUnenroll struct {
		Agents []string `json:"agents"`
		Force  bool     `json:"force"`
	}
	if err := json.NewEncoder(&body).Encode(bulkUnenroll{agentIDs, force}); err != nil {
		return err
	}
	req := c.newFleetRequest("POST", "/agents/bulk_unenroll", &body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	return nil
}

// AgentPolicies returns the Agent Policies matching the given KQL query.
func (c *Client) AgentPolicies(kuery string) ([]AgentPolicy, error) {
	u, err := url.Parse(c.fleetURL + "/agent_policies")
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Add("kuery", kuery)
	u.RawQuery = query.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	var result struct {
		Items []AgentPolicy `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// DeleteAgentPolicy deletes the Agent Policy with the given ID.
func (c *Client) DeleteAgentPolicy(id string) error {
	var body bytes.Buffer
	type deleteAgentPolicy struct {
		ID string `json:"agentPolicyId"`
	}
	if err := json.NewEncoder(&body).Encode(deleteAgentPolicy{id}); err != nil {
		return err
	}
	req := c.newFleetRequest("POST", "/agent_policies/delete", &body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	return nil
}

// CreateAgentPolicy returns the default Agent Policy.
func (c *Client) CreateAgentPolicy(name, namespace, description string) (*AgentPolicy, *EnrollmentAPIKey, error) {
	var body bytes.Buffer
	type newAgentPolicy struct {
		Name        string `json:"name,omitempty"`
		Namespace   string `json:"namespace,omitempty"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewEncoder(&body).Encode(newAgentPolicy{name, namespace, description}); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", c.fleetURL+"/agent_policies", &body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kbn-xsrf", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	var result struct {
		Item AgentPolicy `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}
	enrollmentAPIKey, err := c.getAgentPolicyEnrollmentAPIKey(result.Item.ID)
	if err != nil {
		return nil, nil, err
	}
	return &result.Item, enrollmentAPIKey, nil
}

func (c *Client) getAgentPolicyEnrollmentAPIKey(policyID string) (*EnrollmentAPIKey, error) {
	keys, err := c.enrollmentAPIKeys("fleet-enrollment-api-keys.policy_id:" + policyID)
	if err != nil {
		return nil, err
	}
	if n := len(keys); n != 1 {
		return nil, fmt.Errorf("expected 1 enrollment API key, got %d", n)
	}
	resp, err := http.Get(c.fleetURL + "/enrollment-api-keys/" + keys[0].ID)
	if err != nil {
		return nil, err
	}
	var result struct {
		Item EnrollmentAPIKey `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Item, nil
}

func (c *Client) enrollmentAPIKeys(kuery string) ([]EnrollmentAPIKey, error) {
	u, err := url.Parse(c.fleetURL + "/enrollment-api-keys")
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Add("kuery", kuery)
	u.RawQuery = query.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	var result struct {
		Items []EnrollmentAPIKey `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListPackages lists all packages available for installation.
func (c *Client) ListPackages() ([]Package, error) {
	resp, err := http.Get(c.fleetURL + "/epm/packages?experimental=true")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Response []Package `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Response, nil
}

// PackagePolicy returns information about the package policy with the given ID.
func (c *Client) PackagePolicy(id string) (*PackagePolicy, error) {
	resp, err := http.Get(c.fleetURL + "/package_policies/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	var result struct {
		Item PackagePolicy `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Item, nil
}

// CreatePackagePolicy adds an integration to a policy.
func (c *Client) CreatePackagePolicy(p PackagePolicy) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(&p); err != nil {
		return err
	}
	req := c.newFleetRequest("POST", "/package_policies", &body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	return nil
}

// DeletePackagePolicy deletes one or more package policies.
func (c *Client) DeletePackagePolicy(ids ...string) error {
	var params struct {
		PackagePolicyIDs []string `json:"packagePolicyIds"`
	}
	params.PackagePolicyIDs = ids
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(params); err != nil {
		return err
	}
	req := c.newFleetRequest("POST", "/package_policies/delete", &body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed (%s): %s", resp.Status, body)
	}
	return nil
}

func (c *Client) newFleetRequest(method string, path string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, c.fleetURL+path, body)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kbn-xsrf", "1")
	return req
}
