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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

const (
	activeBranchesAPI = "https://storage.googleapis.com/artifacts-api/snapshots/branches.json"
)

type activeBranchesResp struct {
	Branches []string `json:"branches"`
}

// queryActiveBranches queries the Elastic active branches API to get the
// active branches in Elastic repositories.
func queryActiveBranches(ctx context.Context) ([]string, error) {
	var httpClient http.Client

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, activeBranchesAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create http request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}

	var activeBranches activeBranchesResp
	if err = json.Unmarshal(b, &activeBranches); err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return activeBranches.Branches, nil
}

// convertActiveBranchesToVersions converts active branches to versions.
func convertActiveBranchesToVersions(activeBranches []string) ([]string, error) {
	// Active branch is either 'main' or some minor version in the form of 'x.y',
	// and the branches should be in chronological order, i.e. 'main' is last.
	versions := make([]string, 0, len(activeBranches))
	for _, activeBranch := range activeBranches {
		// If it is not 'main', simply add it to versions.
		if activeBranch != "main" {
			versions = append(versions, activeBranch)
			continue
		}
		// Otherwise, we replace 'main' with the latest version 'x.y+1'.
		prevVersion := versions[len(versions)-1]
		splits := strings.Split(prevVersion, ".")
		minor, err := strconv.ParseInt(splits[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version %s: %w", prevVersion, err)
		}
		versions = append(versions, fmt.Sprintf("%s.%d", splits[0], minor+1))
	}

	return versions, nil
}

func getTestSnapshotVersions(ctx context.Context, vsCache *ech.VersionsCache) (ech.Versions, error) {
	activeBranches, err := queryActiveBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query active branches: %w", err)
	}

	activeVersions, err := convertActiveBranchesToVersions(activeBranches)
	if err != nil {
		return nil, fmt.Errorf("failed to convert active branches to versions: %w", err)
	}

	var snapshots ech.Versions
	for _, v := range activeVersions {
		snapshot, err := vsCache.GetLatestSnapshot(v)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}
