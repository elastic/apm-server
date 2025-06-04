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

// Package kibana implements a Kibana HTTP API client to perform operation needed by functional tests.
package kibana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/elastic/apm-server/functionaltests/internal/ech"
)

func NewClient(kibanaURL, username, password string) (*Client, error) {
	if kibanaURL == "" {
		return nil, fmt.Errorf("kibana.NewClient kibanaURL must not be empty")
	}

	return &Client{
		url:                 kibanaURL,
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

	// Fleet API access require superuser role before 8.1.0.
	// superUsername should be an Elasticsearch superuser username.
	superUsername string
	// superPassword should be an Elasticsearch superuser password.
	superPassword string

	SupportedAPIVersion string
}

// prepareRequest creates a http.Request with required headers for interacting with Kibana.
func (c *Client) prepareRequest(method, path string, body any) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal body: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.url, path)
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("cannot create http request: %w", err)
	}

	req.Header.Add("kbn-xsrf", "true")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Elastic-Api-Version", c.SupportedAPIVersion)
	req.Header.Add("X-Elastic-Internal-Origin", "Kibana")

	userPass := fmt.Sprintf("%s:%s", c.superUsername, c.superPassword)
	basicAuth := base64.StdEncoding.EncodeToString([]byte(userPass))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", basicAuth))

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
	handleRespError func(statusCode int, body []byte) error,
) ([]byte, error) {
	methodPath := fmt.Sprintf("%s %s", method, path)
	req, err := c.prepareRequest(method, path, body)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare request (%s): %w", methodPath, err)
	}

	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request (%s): %w", methodPath, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body (%s): %w", methodPath, err)
	}

	if resp.StatusCode != 200 {
		if handleRespError != nil {
			if err = handleRespError(resp.StatusCode, b); err != nil {
				return nil, err
			}
		}
		errMsg := fmt.Sprintf("request (%s) failed with status code %d", methodPath, resp.StatusCode)
		if len(b) > 0 {
			return nil, fmt.Errorf("%s, body: %s", errMsg, string(b))
		}
		return nil, errors.New(errMsg)
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
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Package     PackagePolicyPkg     `json:"package"`
	Namespace   string               `json:"namespace"`
	Inputs      []PackagePolicyInput `json:"inputs"`
	OutputID    string               `json:"output_id"`
	PolicyID    string               `json:"policy_id,omitempty"`
	PolicyIDs   []string             `json:"policy_ids,omitempty"`
	Enabled     bool                 `json:"enabled"`
}

type PackagePolicyPkg struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type PackagePolicyInput struct {
	Config         any            `json:"config"`
	Enabled        bool           `json:"enabled"`
	Streams        []any          `json:"streams"`
	Type           string         `json:"type"`
	Vars           map[string]any `json:"vars"`
	KeepEnabled    bool           `json:"keep_enabled"`
	PolicyTemplate string         `json:"policy_template"`
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

	b, err := c.sendRequest(ctx, http.MethodGet, path, nil, handleRespError)
	if err != nil {
		return PackagePolicy{}, err
	}

	var policyResp getPackagePolicyResponse
	if err = json.Unmarshal(b, &policyResp); err != nil {
		return PackagePolicy{}, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return policyResp.Item, nil
}

// UpdatePackagePolicyByID performs a Package Policy update in Fleet through the Fleet Kibana APIs.
// https://www.elastic.co/docs/api/doc/kibana/v8/operation/operation-update-package-policy
func (c *Client) UpdatePackagePolicyByID(ctx context.Context, policyID string, policy PackagePolicy) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/fleet/package_policies/%s", policyID)
	_, err := c.sendRequest(ctx, http.MethodPut, path, policy, nil)
	return err
}

// UpdatePackagePolicyDescriptionByID updates a Package Policy description through the Fleet Kibana APIs.
func (c *Client) UpdatePackagePolicyDescriptionByID(
	ctx context.Context,
	policyID string,
	version ech.Version,
	description string,
) error {
	policy, err := c.GetPackagePolicyByID(ctx, policyID)
	if err != nil {
		return fmt.Errorf("cannot get elastic-cloud-apm package policy: %w", err)
	}

	// If the package policy version returned from API does not match with
	// expected version, set it ourselves to hopefully circumvent it.
	// Relevant issue: https://github.com/elastic/kibana/issues/215437.
	//
	// NOTE: We check that the version in the package policy has the same
	// Major.Minor version as we expect (instead of whole version string),
	// because 7.17.x somehow has package policy version of 7.17.0.
	if !strings.HasPrefix(policy.Package.Version, version.MajorMinor()) {
		// Set the expected version for this update.
		policy.Package.Version = version.String()
	}

	policy.Description = description
	if err = c.UpdatePackagePolicyByID(ctx, policyID, policy); err != nil {
		return fmt.Errorf("cannot update elastic-cloud-apm package policy: %w", err)
	}
	return nil
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
	b, err := c.sendRequest(ctx, http.MethodPost, path, nil, nil)
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

/* Upgrade Assistant APIs */

func (c *Client) ResolveMigrationDeprecations(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := c.migrateSystemIndicesAndWait(ctx); err != nil {
		return err
	}

	deprecations, err := c.QueryCriticalESDeprecations(ctx)
	if err != nil {
		return fmt.Errorf("failed to query critical deprecations: %w", err)
	}

	var errs []error
	for _, deprecation := range deprecations {
		switch deprecation.Type {
		case "index_settings":
			errs = append(errs, c.markIndexAsReadOnly(ctx, deprecation.Name))
		case "data_streams":
			errs = append(errs, c.markDataStreamAsReadOnly(
				ctx,
				deprecation.Name,
				deprecation.CorrectiveAction.Metadata.IndicesRequiringUpgrade,
			))
		default:
			errs = append(errs, fmt.Errorf("unknown deprecation type: %s", deprecation.Type))
		}
	}

	return errors.Join(errs...)
}

type MigrationDeprecation struct {
	Name             string `json:"index"`
	Type             string `json:"type"`
	IsCritical       bool   `json:"isCritical"`
	Level            string `json:"level"`
	CorrectiveAction struct {
		Type     string `json:"type"`
		Metadata struct {
			IndicesRequiringUpgrade []string `json:"indicesRequiringUpgrade,omitempty"`
		}
	} `json:"correctiveAction"`
}

func (d MigrationDeprecation) isLevelCritical() bool {
	// Older Kibana uses "isCritical" while newer Kibana uses "level".
	return d.IsCritical || strings.EqualFold(d.Level, "critical")
}

type esDeprecationsResponse struct {
	MigrationDeprecations []MigrationDeprecation `json:"migrationsDeprecations"`
}

// QueryCriticalESDeprecations retrieves the critical deprecation warnings for Elasticsearch.
// It is essentially equivalent to `GET _migration/deprecations`, but through Kibana Upgrade
// Assistant API.
func (c *Client) QueryCriticalESDeprecations(ctx context.Context) ([]MigrationDeprecation, error) {
	path := "/api/upgrade_assistant/es_deprecations"
	b, err := c.sendRequest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var esDeprecationsResp esDeprecationsResponse
	if err = json.Unmarshal(b, &esDeprecationsResp); err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	// Remove all non-critical deprecation info.
	return slices.DeleteFunc(
		esDeprecationsResp.MigrationDeprecations,
		func(dep MigrationDeprecation) bool {
			return !dep.isLevelCritical()
		},
	), nil
}

type upgradeAssistUpdateIndexRequest struct {
	Operations []string `json:"operations"`
}

// markIndexAsReadOnly updates the index to read-only through the Upgrade Assistant API:
// https://www.elastic.co/guide/en/kibana/current/upgrade-assistant.html.
func (c *Client) markIndexAsReadOnly(ctx context.Context, index string) error {
	path := fmt.Sprintf("/api/upgrade_assistant/update_index/%s", index)
	req := upgradeAssistUpdateIndexRequest{
		Operations: []string{"blockWrite", "unfreeze"},
	}

	_, err := c.sendRequest(ctx, http.MethodPost, path, req, nil)
	return err
}

type upgradeAssistMigrateDSRequest struct {
	Indices []string `json:"indices"`
}

// markDataStreamAsReadOnly marks the backing indices of the data stream as read-only
// through the Upgrade Assistant API:
// https://www.elastic.co/guide/en/kibana/current/upgrade-assistant.html.
func (c *Client) markDataStreamAsReadOnly(ctx context.Context, dataStream string, indices []string) error {
	// Data stream
	path := fmt.Sprintf("/api/upgrade_assistant/migrate_data_stream/%s/readonly", dataStream)
	req := upgradeAssistMigrateDSRequest{
		Indices: indices,
	}

	_, err := c.sendRequest(ctx, http.MethodPost, path, req, nil)
	return err
}

type SystemIndicesMigrationStatus string

const (
	MigrationNeeded   SystemIndicesMigrationStatus = "MIGRATION_NEEDED"
	NoMigrationNeeded SystemIndicesMigrationStatus = "NO_MIGRATION_NEEDED"
	InProgress        SystemIndicesMigrationStatus = "IN_PROGRESS"
)

type systemIndicesMigrationResponse struct {
	MigrationStatus string `json:"migration_status"`
}

// QuerySystemIndicesMigrationStatus returns the system indices migration status retrieved
// through the Upgrade Assistant API:
// https://www.elastic.co/guide/en/kibana/current/upgrade-assistant.html.
func (c *Client) QuerySystemIndicesMigrationStatus(ctx context.Context) (SystemIndicesMigrationStatus, error) {
	path := "/api/upgrade_assistant/system_indices_migration"
	b, err := c.sendRequest(ctx, http.MethodGet, path, nil, nil)

	var systemIndicesMigrationResp systemIndicesMigrationResponse
	if err = json.Unmarshal(b, &systemIndicesMigrationResp); err != nil {
		return "", fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	status := SystemIndicesMigrationStatus(systemIndicesMigrationResp.MigrationStatus)
	return status, nil
}

// migrateSystemIndices migrates the system indices through the Upgrade Assistant API:
// https://www.elastic.co/guide/en/kibana/current/upgrade-assistant.html.
func (c *Client) migrateSystemIndices(ctx context.Context) error {
	path := "/api/upgrade_assistant/system_indices_migration"
	_, err := c.sendRequest(ctx, http.MethodPost, path, nil, nil)
	return err
}

// migrateSystemIndicesAndWait first checks that the system indices need to be migrated.
// If they do, it will call migrateSystemIndices and wait until the migration is done.
func (c *Client) migrateSystemIndicesAndWait(ctx context.Context) error {
	status, err := c.QuerySystemIndicesMigrationStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to query system indices migration status: %w", err)
	}

	// Migration needed, call migrate API.
	if status == MigrationNeeded {
		if err = c.migrateSystemIndices(ctx); err != nil {
			return fmt.Errorf("failed to migrate system indices: %w", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			status, err = c.QuerySystemIndicesMigrationStatus(ctx)
			if err != nil {
				return fmt.Errorf("failed to query system indices migration status: %w", err)
			}
			// Migration done.
			if status == NoMigrationNeeded {
				return nil
			}
			// Migration in progress, wait for a while.
			time.Sleep(5 * time.Second)
		}
	}
}
