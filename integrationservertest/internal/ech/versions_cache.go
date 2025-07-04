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
)

func NewVersionsCache(ctx context.Context, client *Client, ecRegion string) (*VersionsCache, error) {
	cs, err := client.GetCandidateVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	ss, err := client.GetSnapshotVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	vs, err := client.GetVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	upgradeInfo := map[Version]Versions{}
	candidates := unpackStackVersions(cs, upgradeInfo)
	snapshots := unpackStackVersions(ss, upgradeInfo)
	versions := unpackStackVersions(vs, upgradeInfo)

	return &VersionsCache{
		fetchedCandidates: candidates,
		fetchedSnapshots:  snapshots,
		fetchedVersions:   versions,
		upgradeInfo:       upgradeInfo,
		region:            ecRegion,
	}, nil
}

func unpackStackVersions(
	stackVersions []StackVersion,
	upgradeInfo map[Version]Versions,
) Versions {
	versions := Versions{}
	for _, stackVersion := range stackVersions {
		versions = append(versions, stackVersion.Version)
		upgradeInfo[stackVersion.Version] = stackVersion.UpgradableTo
	}
	return versions
}

// VersionsCache is used to cache stack versions that are fetched from Elastic Cloud API,
// in order to avoid redundant HTTP requests each time we need the versions.
// It also includes some helper functions to more easily obtain the latest version given
// a particular prefix.
type VersionsCache struct {
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates Versions
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots Versions
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions Versions

	upgradeInfo map[Version]Versions

	region string
}

func (c *VersionsCache) GetUpgradeToVersions(from Version) Versions {
	upgradableVersions := c.upgradeInfo[from]
	return upgradableVersions
}

func (c *VersionsCache) CanUpgradeTo(from, to Version) bool {
	upgradableVersions := c.GetUpgradeToVersions(from)
	return upgradableVersions.Has(to)
}

// GetLatestSnapshot retrieves the latest snapshot version for the version prefix.
func (c *VersionsCache) GetLatestSnapshot(t *testing.T, prefix string) Version {
	t.Helper()
	ver, ok := c.fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "snapshot for '%s' not found in EC region %s", prefix, c.region)
	return ver
}

// GetLatestVersion retrieves the latest non-snapshot version for the version prefix.
func (c *VersionsCache) GetLatestVersion(t *testing.T, prefix string) Version {
	t.Helper()
	ver, ok := c.fetchedVersions.LatestFor(prefix)
	require.True(t, ok, "version for '%s' not found in EC region %s", prefix, c.region)
	return ver
}
