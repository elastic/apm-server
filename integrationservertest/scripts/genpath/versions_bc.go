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
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

const (
	// NOTE: This API may change in the future and not be accessible eventually.
	futureReleasesAPI = "https://artifacts.elastic.co/releases/TfEVhiaBGqR64ie0g0r0uUwNAbEQMu1Z/future-releases/stack.json"
)

type futureReleasesResp struct {
	Releases []release `json:"releases"`
}

type release struct {
	Version           string         `json:"version"`
	FeatureFreezeDate string         `json:"feature_freeze_date"`
	ActiveRelease     bool           `json:"active_release"`
	BuildCandidates   map[string]any `json:"build_candidates"`
}

// queryFutureReleases queries the Elastic future releases API to get the
// active build candidates.
func queryFutureReleases(ctx context.Context) ([]release, error) {
	var httpClient http.Client

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, futureReleasesAPI, nil)
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

	var futureReleases futureReleasesResp
	if err = json.Unmarshal(b, &futureReleases); err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %w", err)
	}

	return futureReleases.Releases, nil
}

// filterReleasesForBCs filters the list of releases to get only build candidates.
func filterReleasesForBCs(releases []release) []string {
	var result []string
	for _, r := range releases {
		// Ignore non-active releases.
		if !r.ActiveRelease {
			continue
		}
		// Ignore versions without feature freeze date since it's not ready yet.
		if r.FeatureFreezeDate == "" {
			continue
		}
		// Ignore versions without build candidates because that's what we want.
		if len(r.BuildCandidates) == 0 {
			continue
		}
		result = append(result, r.Version)
	}
	return result
}

func getTestBCVersions(ctx context.Context, vsCache *ech.VersionsCache) (ech.Versions, error) {
	releases, err := queryFutureReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query future releases: %w", err)
	}

	var bcs ech.Versions
	for _, v := range filterReleasesForBCs(releases) {
		bc, err := vsCache.GetLatestVersion(v)
		if err != nil {
			if errors.Is(err, ech.ErrNotFoundInEC) {
				log.Printf("skipping version '%s' since it is not found\n", v)
				continue
			}
			return nil, fmt.Errorf("failed to get latest version: %w", err)
		}
		bcs = append(bcs, bc)
	}

	return bcs, nil
}
