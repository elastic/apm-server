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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"
)

func (c *Client) ResolveMigrationDeprecations(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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
	CorrectiveAction struct {
		Type     string `json:"type"`
		Metadata struct {
			IndicesRequiringUpgrade []string `json:"indicesRequiringUpgrade,omitempty"`
		}
	} `json:"correctiveAction"`
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
			return !dep.IsCritical
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
