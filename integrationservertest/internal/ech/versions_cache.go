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
)

var (
	ErrVersionNotFoundInEC = errors.New("version not found in EC")
)

func NewVersionsCache(ctx context.Context, client *Client, ecRegion string) (*VersionsCache, error) {
	ss, err := client.GetSnapshotVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	vs, err := client.GetVersions(ctx, ecRegion)
	if err != nil {
		return nil, err
	}

	upgradeTo := map[Version]Versions{}
	upgradeFrom := map[Version]Versions{}
	snapshots := unpackStackVersions(ss, upgradeTo, upgradeFrom)
	versions := unpackStackVersions(vs, upgradeTo, upgradeFrom)

	return &VersionsCache{
		fetchedSnapshots: snapshots,
		fetchedVersions:  versions,
		upgradeTo:        upgradeTo,
		upgradeFrom:      upgradeFrom,
		region:           ecRegion,
	}, nil
}

func unpackStackVersions(
	stackVersions []StackVersion,
	upgradeTo map[Version]Versions,
	upgradeFrom map[Version]Versions,
) Versions {
	versions := Versions{}
	for _, stackVersion := range stackVersions {
		versions = append(versions, stackVersion.Version)
		upgradeTo[stackVersion.Version] = stackVersion.UpgradableTo
		for _, v := range stackVersion.UpgradableTo {
			upgradeFrom[v] = append(upgradeFrom[v], stackVersion.Version)
		}
	}
	return versions
}

// VersionsCache is used to cache stack versions that are fetched from Elastic Cloud API,
// in order to avoid redundant HTTP requests each time we need the versions.
// It also includes some helper functions to more easily obtain the latest version given
// a particular prefix.
type VersionsCache struct {
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots Versions
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions Versions

	// upgradeTo is a map of version to the versions that it can upgrade to,
	// i.e. 'key' -> list of versions that 'key' can upgrade to.
	upgradeTo map[Version]Versions
	// upgradeFrom is a map of version to the versions that it can upgrade from,
	// i.e. 'key' -> list of versions that can upgrade to 'key'.
	upgradeFrom map[Version]Versions

	region string
}

func (c *VersionsCache) GetUpgradeToVersions(from Version) Versions {
	upgradeToVersions := c.upgradeTo[from]
	return upgradeToVersions
}

func (c *VersionsCache) GetUpgradeFromVersions(to Version) Versions {
	upgradeFromVersions := c.upgradeFrom[to]
	return upgradeFromVersions
}

func (c *VersionsCache) CanUpgrade(from, to Version) bool {
	upgradableVersions := c.GetUpgradeToVersions(from)
	return upgradableVersions.Has(to)
}

// GetLatestSnapshot retrieves the latest snapshot version for the version prefix.
func (c *VersionsCache) GetLatestSnapshot(prefix string) (Version, error) {
	ver, ok := c.fetchedSnapshots.LatestFor(prefix)
	if !ok {
		return Version{}, fmt.Errorf("snapshot '%s' for region %s: %w", prefix, c.region, ErrVersionNotFoundInEC)
	}
	return ver, nil
}

// GetLatestVersion retrieves the latest non-snapshot version for the version prefix.
func (c *VersionsCache) GetLatestVersion(prefix string) (Version, error) {
	ver, ok := c.fetchedVersions.LatestFor(prefix)
	if !ok {
		return Version{}, fmt.Errorf("version '%s' for region %s: %w", prefix, c.region, ErrVersionNotFoundInEC)
	}
	return ver, nil
}
