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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/version"
)

func NewVersionsCache(ctx context.Context, ecc *Client, ecRegion string) (*VersionsCache, error) {
	cs, err := ecc.GetCandidateVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	ss, err := ecc.GetSnapshotVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	vs, err := ecc.GetVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	upgradeInfo := map[version.Version]version.Versions{}
	candidates := unpackStackVersions(cs, upgradeInfo)
	snapshots := unpackStackVersions(ss, upgradeInfo)
	versions := unpackStackVersions(vs, upgradeInfo)

	return &VersionsCache{
		fetchedCandidates: candidates,
		fetchedSnapshots:  snapshots,
		fetchedVersions:   versions,
		region:            ecRegion,
	}, nil
}

func unpackStackVersions(
	stackVersions []StackVersion,
	upgradeInfo map[version.Version]version.Versions,
) version.Versions {
	versions := version.Versions{}
	for _, stackVersion := range stackVersions {
		versions = append(versions, stackVersion.Version)
		upgradeInfo[stackVersion.Version] = stackVersion.UpgradableTo
	}
	return versions
}

type VersionsCache struct {
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates version.Versions
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots version.Versions
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions version.Versions

	upgradeInfo map[version.Version]version.Versions

	region string
}

func (c *VersionsCache) CanUpgradeTo(from, to version.Version) bool {
	upgradableVersions := c.upgradeInfo[from]
	return upgradableVersions.Has(to)
}

// GetLatestSnapshot retrieves the latest snapshot version for the version prefix.
func (c *VersionsCache) GetLatestSnapshot(t *testing.T, prefix string) version.Version {
	t.Helper()
	ver, ok := c.fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "snapshot for '%s' found in EC region %s", prefix, c.region)
	return ver
}

// GetLatestVersionOrSkip retrieves the latest non-snapshot version for the version prefix.
// If the version is not found, the test is skipped.
func (c *VersionsCache) GetLatestVersionOrSkip(t *testing.T, prefix string) version.Version {
	t.Helper()
	ver, ok := c.fetchedVersions.LatestFor(prefix)
	if !ok {
		t.Skipf("version for '%s' not found in EC region %s, skipping test", prefix, c.region)
		return version.Version{}
	}
	return ver
}

// GetLatestBCOrSkip retrieves the latest build-candidate version for the version prefix.
// If the version is not found, the test is skipped.
func (c *VersionsCache) GetLatestBCOrSkip(t *testing.T, prefix string) version.Version {
	t.Helper()
	candidate, ok := c.fetchedCandidates.LatestFor(prefix)
	if !ok {
		t.Skipf("BC for '%s' not found in EC region %s, skipping test", prefix, c.region)
		return version.Version{}
	}

	// Check that the BC version is actually latest, otherwise skip the test.
	versionInfo := c.GetLatestVersionOrSkip(t, prefix)
	if versionInfo.Major != candidate.Major {
		t.Skipf("BC for '%s' is invalid in EC region %s, skipping test", prefix, c.region)
		return version.Version{}
	}
	if versionInfo.Minor > candidate.Minor {
		t.Skipf("BC for '%s' is less than latest normal version in EC region %s, skipping test",
			prefix, c.region)
		return version.Version{}
	}

	return candidate
}
