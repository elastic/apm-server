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

package ech

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/cloud-sdk-go/pkg/client/deployments"
	"github.com/elastic/cloud-sdk-go/pkg/client/stack"
	"github.com/elastic/cloud-sdk-go/pkg/models"
)

type Client struct {
	ecAPI    *api.API
	endpoint string
}

type clientConfig struct {
	httpClient *http.Client
}

func (o *clientConfig) initDefaults() {
	o.httpClient = new(http.Client)
}

type ClientOption func(*clientConfig)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(o *clientConfig) {
		o.httpClient = httpClient
	}
}

func NewClient(endpoint string, apiKey string, options ...ClientOption) (*Client, error) {
	cfg := clientConfig{}
	cfg.initDefaults()
	for _, o := range options {
		o(&cfg)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("ech.NewClient apiKey is required")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("ech.NewClient endpoint is required")
	}

	ecAPI, err := api.NewAPI(api.Config{
		AuthWriter: auth.APIKey(apiKey),
		Client:     cfg.httpClient,
		Host:       endpoint,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create Elastic Cloud API client: %w", err)
	}

	return &Client{
		ecAPI:    ecAPI,
		endpoint: endpoint,
	}, nil
}

func (c *Client) RestartIntegrationServer(ctx context.Context, deploymentID string) error {
	res, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          c.ecAPI,
		DeploymentID: deploymentID,
	})
	if err != nil {
		return fmt.Errorf("cannot retrieve ref id of integrations server for deployment %s: %w", deploymentID, err)
	}

	refID := *res.Resources.IntegrationsServer[0].RefID

	// https://www.elastic.co/docs/api/doc/cloud/operation/operation-restart-deployment-stateless-resource
	url := fmt.Sprintf("%s/api/v1/deployments/%s/integrations_server/%s/_restart", c.endpoint, deploymentID, refID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("cannot create integrations server restart request for deployment %s: %w", deploymentID, err)
	}

	req = c.ecAPI.AuthWriter.AuthRequest(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot execute HTTP request for restarting deployment %s: %w", deploymentID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("cannot read body after receiving a %d response while restarting integrations server: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("restarting integrations server returned %d response with content: %s", resp.StatusCode, b)
	}

	// Wait until the integration server is back online.
	status := func() (string, error) {
		r, err := c.ecAPI.V1API.Deployments.GetDeploymentIntegrationsServerResourceInfo(
			deployments.NewGetDeploymentIntegrationsServerResourceInfoParams().
				WithDeploymentID(deploymentID).
				WithRefID(refID),
			c.ecAPI.AuthWriter)
		if err != nil {
			return "", err
		}

		return *r.Payload.Info.Status, nil
	}
	timeout := 10 * time.Minute
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		select {
		case <-tctx.Done():
			return fmt.Errorf("timeout reached waiting for integrations server to restart")
		case <-ticker.C:
			s, err := status()
			if err != nil {
				return fmt.Errorf("cannot retrieve integrations server status: %w", err)
			}
			if s == "started" {
				return nil
			}
		}
	}
}

type StackVersion struct {
	Version      Version
	UpgradableTo []Version
}

// CanUpgradeTo checks if the current stack version can upgrade to the provided `version`.
func (stackVer StackVersion) CanUpgradeTo(version Version) bool {
	for _, upgrade := range stackVer.UpgradableTo {
		if upgrade == version {
			return true
		}
	}
	return false
}

func (c *Client) getVersions(
	ctx context.Context,
	region string,
	showUnusable bool,
	preFilter func(*models.StackVersionConfig) bool, // Filter before conversion
	postFilter func(Version) bool, // Filter after conversion
) ([]StackVersion, error) {
	showDeleted := false
	resp, err := c.ecAPI.V1API.Stack.GetVersionStacks(
		// Add region to get the stack versions for that region only
		stack.NewGetVersionStacksParamsWithContext(api.WithRegion(ctx, region)).
			WithShowDeleted(&showDeleted).
			WithShowUnusable(&showUnusable),
		c.ecAPI.AuthWriter,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve stack versions: %w", err)
	}

	if resp.Payload == nil || len(resp.Payload.Stacks) == 0 {
		return nil, errors.New("stack versions response payload is empty")
	}

	stackVersions := make([]StackVersion, 0, len(resp.Payload.Stacks))
	for _, s := range resp.Payload.Stacks {
		if preFilter != nil && !preFilter(s) {
			continue
		}
		v, err := NewVersionFromString(s.Version)
		if err != nil {
			return nil, fmt.Errorf("cannot parse stack version '%v': %w", s.Version, err)
		}
		if postFilter == nil || postFilter(v) {
			upgradableTo, err := sortedStackVersionsForStrs(s.UpgradableTo)
			if err != nil {
				return nil, fmt.Errorf("cannot parse upgradable to for '%v': %w", s.Version, err)
			}
			stackVersions = append(stackVersions, StackVersion{
				Version:      v,
				UpgradableTo: upgradableTo,
			})
		}
	}

	slices.SortFunc(stackVersions, func(a, b StackVersion) int {
		return a.Version.Compare(b.Version)
	})
	return stackVersions, nil
}

func sortedStackVersionsForStrs(strs []string) ([]Version, error) {
	versions := make(Versions, 0, len(strs))
	for _, s := range strs {
		v, err := NewVersionFromString(s)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	versions.Sort()
	return versions, nil
}

// GetVersions retrieves all stack version infos without suffix.
func (c *Client) GetVersions(ctx context.Context, region string) ([]StackVersion, error) {
	postFilter := func(v Version) bool {
		// Ignore all with suffix e.g. SNAPSHOTS, BC1
		return v.Suffix == ""
	}

	versions, err := c.getVersions(ctx, region, false, nil, postFilter)
	if err != nil {
		return nil, fmt.Errorf("get versions failed: %w", err)
	}
	return versions, nil
}

// GetSnapshotVersions retrieves all stack version infos with the suffix "SNAPSHOT".
func (c *Client) GetSnapshotVersions(ctx context.Context, region string) ([]StackVersion, error) {
	postFilter := func(v Version) bool {
		// Only keep SNAPSHOTs
		return v.IsSnapshot()
	}

	versions, err := c.getVersions(ctx, region, true, nil, postFilter)
	if err != nil {
		return nil, fmt.Errorf("get snapshot versions failed: %w", err)
	}
	return versions, nil
}
