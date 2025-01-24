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

package ecclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/elastic/cloud-sdk-go/pkg/api"
	"github.com/elastic/cloud-sdk-go/pkg/api/deploymentapi"
	"github.com/elastic/cloud-sdk-go/pkg/auth"
	"github.com/elastic/cloud-sdk-go/pkg/client/deployments"
)

type Client struct {
	*api.API
	endpoint string
}

func New(endpoint string) (*Client, error) {
	apiKey := os.Getenv("EC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("unable to obtain value from EC_API_KEY environment variable")
	}

	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	ess, err := api.NewAPI(api.Config{
		AuthWriter: auth.APIKey(apiKey),
		Client:     new(http.Client),
		Host:       endpoint,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create Elastic Cloud API client: %w", err)
	}

	c := Client{ess, endpoint}

	return &c, nil
}

func (c *Client) RestartIntegrationServer(ctx context.Context, deploymentID string) error {
	res, err := deploymentapi.Get(deploymentapi.GetParams{
		API:          c.API,
		DeploymentID: deploymentID,
	})
	if err != nil {

		return fmt.Errorf("cannot retrieve ref id of integrations server for deployment %s: %w", deploymentID, err)
	}

	refID := *res.Resources.IntegrationsServer[0].RefID

	// This is an undocumented API, but it works.
	// Is like https://www.elastic.co/docs/api/doc/cloud/operation/operation-restart-deployment-es-resource
	// but using integrations_server instead of elasticsearch. integrations_server is the expected Kind for
	// the 8.x APM server setup on Elastic Cloud.
	url := fmt.Sprintf("%s/api/v1/deployments/%s/integrations_server/%s/_restart", c.endpoint, deploymentID, refID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("cannot create integrations server restart request for deployment %s: %w", deploymentID, err)
	}

	req = c.API.AuthWriter.AuthRequest(req)
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
		r, err := c.API.V1API.Deployments.GetDeploymentIntegrationsServerResourceInfo(
			deployments.NewGetDeploymentIntegrationsServerResourceInfoParams().
				WithDeploymentID(deploymentID).
				WithRefID(refID),
			c.API.AuthWriter)
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
