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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

var (
	// fetchedCandidates are the build-candidate stack versions prefetched from Elastic Cloud API.
	fetchedCandidates ecclient.StackVersionInfos
	// fetchedSnapshots are the snapshot stack versions prefetched from Elastic Cloud API.
	fetchedSnapshots ecclient.StackVersionInfos
	// fetchedVersions are the non-snapshot stack versions prefetched from Elastic Cloud API.
	fetchedVersions ecclient.StackVersionInfos
)

// getLatestVersionOrSkip retrieves the latest non-snapshot version for the version prefix.
// If the version is not found, the test is skipped via t.Skip.
func getLatestVersionOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := fetchedVersions.LatestFor(prefix)
	if !ok {
		t.Skipf("version for '%s' not found in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}
	return version
}

// getLatestBCOrSkip retrieves the latest build-candidate version for the version prefix.
// If the version is not found, the test is skipped via t.Skip.
func getLatestBCOrSkip(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	candidate, ok := fetchedCandidates.LatestFor(prefix)
	if !ok {
		t.Skipf("BC for '%s' not found in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}

	// Check that the BC version is actually latest, otherwise skip test.
	versionInfo := getLatestVersionOrSkip(t, prefix)
	if versionInfo.Version.Major != candidate.Version.Major {
		t.Skipf("BC for '%s' is invalid in EC region %s, skipping test", prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}
	if versionInfo.Version.Minor > candidate.Version.Minor {
		t.Skipf("BC for '%s' is less than latest normal version in EC region %s, skipping test",
			prefix, regionFrom(*target))
		return ecclient.StackVersionInfo{}
	}

	return candidate
}

// getLatestSnapshot retrieves the latest snapshot version for the version prefix.
func getLatestSnapshot(t *testing.T, prefix string) ecclient.StackVersionInfo {
	t.Helper()
	version, ok := fetchedSnapshots.LatestFor(prefix)
	require.True(t, ok, "snapshot for '%s' found in EC region %s", prefix, regionFrom(*target))
	return version
}
