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

package functionaltests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

func newVersionsCache(ctx context.Context, ecc *ecclient.Client, ecRegion string) (*versionsCache, error) {
	candidates, err := ecc.GetCandidateVersionInfos(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	snapshots, err := ecc.GetSnapshotVersionInfos(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	versions, err := ecc.GetVersionInfos(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	return &versionsCache{
		fetchedCandidates: candidates,
		fetchedSnapshots:  snapshots,
		fetchedVersions:   versions,
		region:            ecRegion,
	}, nil
}

type versionsCache struct {
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates ecclient.StackVersionInfos
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots ecclient.StackVersionInfos
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions ecclient.StackVersionInfos

	client *ecclient.Client
	region string
}

// GetLatestSnapshot retrieves the latest snapshot version for the version prefix.
func (c *versionsCache) GetLatestSnapshot(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := c.fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "snapshot for '%s' found in EC region %s", prefix, c.region)
	return version
}

// GetLatestVersionOrSkip retrieves the latest non-snapshot version for the version prefix.
// If the version is not found, the test is skipped.
func (c *versionsCache) GetLatestVersionOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := c.fetchedVersions.LatestFor(prefix)
	if !ok {
		t.Skipf("version for '%s' not found in EC region %s, skipping test", prefix, c.region)
		return ecclient.StackVersionInfo{}
	}
	return version
}

// GetLatestBCOrSkip retrieves the latest build-candidate version for the version prefix.
// If the version is not found, the test is skipped.
func (c *versionsCache) GetLatestBCOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	candidate, ok := c.fetchedCandidates.LatestFor(prefix)
	if !ok {
		t.Skipf("BC for '%s' not found in EC region %s, skipping test", prefix, c.region)
		return ecclient.StackVersionInfo{}
	}

	// Check that the BC version is actually latest, otherwise skip the test.
	versionInfo := c.GetLatestVersionOrSkip(t, prefix)
	if versionInfo.Version.Major != candidate.Version.Major {
		t.Skipf("BC for '%s' is invalid in EC region %s, skipping test", prefix, c.region)
		return ecclient.StackVersionInfo{}
	}
	if versionInfo.Version.Minor > candidate.Version.Minor {
		t.Skipf("BC for '%s' is less than latest normal version in EC region %s, skipping test",
			prefix, c.region)
		return ecclient.StackVersionInfo{}
	}

	return candidate
}
