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
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/integrationservertest"
	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

var (
	// target is the Elastic Cloud environment to target with these test.
	// We use 'pro' for production as that is the key used to retrieve EC_API_KEY from secret storage.
	target = flag.String(
		"target",
		"pro",
		"The target environment where to run tests againts. Valid values are: qa, pro.",
	)

	// useSnapshots determines whether the upgrade path to be generated is for SNAPSHOTs.
	useSnapshots = flag.Bool(
		"snapshots",
		false,
		"Use SNAPSHOTs instead of released versions / BCs for tested versions.",
	)

	// majorMinorCutoff determines the version cutoff for testing.
	majorMinorCutoff = flag.String(
		"major-minor-cutoff",
		"8.15",
		"The cutoff version for testing. Only versions greater than or equal to the cutoff will be used for tests.",
	)
)

var versionCutoffMajor, versionCutoffMinor uint64

func parseVersionCutoff() error {
	splits := strings.Split(*majorMinorCutoff, ".")
	if len(splits) != 2 {
		return errors.New("invalid version cutoff")
	}

	var err error
	versionCutoffMajor, err = strconv.ParseUint(splits[0], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse version cutoff major: %w", err)
	}
	versionCutoffMinor, err = strconv.ParseUint(splits[1], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse version cutoff minor: %w", err)
	}

	return nil
}

func versionMeetCutoff(version ech.Version) bool {
	return version.Major > versionCutoffMajor ||
		(version.Major == versionCutoffMajor && version.Minor >= versionCutoffMinor)
}

func main() {
	flag.Parse()

	if err := parseVersionCutoff(); err != nil {
		log.Fatal(err)
	}

	// This is a simple check to alert users if this necessary env var
	// is not available.
	//
	// A valid API key is required to query Elastic Cloud APIs.
	ecAPIKey := os.Getenv("EC_API_KEY")
	if ecAPIKey == "" {
		log.Fatal("EC_API_KEY env var not set")
	}

	ctx := context.Background()
	ecRegion := integrationservertest.RegionFrom(*target)
	ecc, err := ech.NewClient(integrationservertest.EndpointFrom(*target), ecAPIKey)
	if err != nil {
		log.Fatal(err)
	}

	vsCache, err := ech.NewVersionsCache(ctx, ecc, ecRegion)
	if err != nil {
		log.Fatal(err)
	}

	versions, err := fetchTestVersions(ctx, vsCache)
	if err != nil {
		log.Fatal(err)
	}

	upgradePaths := constructUpgradePaths(versions, vsCache)
	// Output as JSON so that it can be decoded onto the workflow YAML.
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(upgradePaths); err != nil {
		log.Fatalf("failed to json marshal upgrade paths: %s", err)
	}
}

func fetchTestVersions(ctx context.Context, vsCache *ech.VersionsCache) (ech.Versions, error) {
	testVersionsFetcher := getTestBCVersions
	if *useSnapshots {
		testVersionsFetcher = getTestSnapshotVersions
	}

	versions, err := testVersionsFetcher(ctx, vsCache)
	if err != nil {
		return nil, err
	}

	return versions.Filter(func(v ech.Version) bool {
		return versionMeetCutoff(v)
	}), nil
}

// getUpgradeFromVersions gets all versions that can upgrade to provided version
// and filter it down to versions appropriate for testing.
func getUpgradeFromVersions(version ech.Version, vsCache *ech.VersionsCache) ech.Versions {
	upgradeFromVersions := vsCache.GetUpgradeFromVersions(version).
		Filter(func(v ech.Version) bool {
			// Filter out versions that don't meet our defined version cutoff.
			// Also filter out versions that has same major-minor as current version,
			// since we don't care about patch upgrades in this test.
			return versionMeetCutoff(v) && v.MajorMinor() != version.MajorMinor()
		})

	// We only care about the latest patch of each major-minor.
	return latestOfEachMajorMinor(upgradeFromVersions)
}

// latestOfEachMajorMinor returns only versions that are the latest patches of
// their respective major-minor.
func latestOfEachMajorMinor(versions ech.Versions) ech.Versions {
	var currMajorMinor string
	result := make(ech.Versions, 0)

	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		if version.MajorMinor() != currMajorMinor {
			result = append(result, version)
			currMajorMinor = version.MajorMinor()
		}
	}

	result.Sort()
	return result
}

// latestOfEachMajor returns only versions that are the latest patches of
// their respective major.
func latestOfEachMajor(versions ech.Versions) ech.Versions {
	var currMajor uint64
	result := make(ech.Versions, 0)

	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		if version.Major != currMajor {
			result = append(result, version)
			currMajor = version.Major
		}
	}

	result.Sort()
	return result
}

type upgradePair struct {
	From, To ech.Version
}

// constructUpgradePaths constructs upgrade paths for provided versions.
func constructUpgradePaths(versions ech.Versions, vsCache *ech.VersionsCache) []string {
	// For each provided version, randomly select some from-versions to form
	// upgrade pairs.
	latestEachMajor := latestOfEachMajor(versions)
	upgradePairs := map[upgradePair]struct{}{}
	for _, to := range versions {
		upgradeFromVersions := getUpgradeFromVersions(to, vsCache)
		// Randomly choose 1 from-version.
		for _, from := range choose(upgradeFromVersions, 1) {
			upgradePairs[upgradePair{from, to}] = struct{}{}
		}
		// For latest versions of each major, we force include minor upgrade and major upgrade.
		if latestEachMajor.Has(to) {
			prevMajorVersion, ok := upgradeFromVersions.LatestForMajor(to.Major - 1)
			if ok {
				upgradePairs[upgradePair{prevMajorVersion, to}] = struct{}{}
			}
			prevMinorVersion, ok := upgradeFromVersions.LatestForMinor(to.Major, to.Minor-1)
			if ok {
				upgradePairs[upgradePair{prevMinorVersion, to}] = struct{}{}
			}
		}
	}

	var upgradeChains [][]upgradePair
	// Merge all upgrade pairs into the longest possible chain.
	// E.g. if we have: X -> Y, Y -> Z, X -> Z, we want to condense it to: X -> Y -> Z, X -> Z.
	for curr := range upgradePairs {
		to := curr.To
		upgradeChain := []upgradePair{curr}
		// Upgrade pair is used up, delete it from map.
		delete(upgradePairs, curr)
		// Find subsequent upgrade pair that can connect to the current one.
		for {
			next, ok := findNextPath(to, upgradePairs)
			if !ok {
				// None found, stop searching and append upgrade path to list.
				upgradeChains = append(upgradeChains, upgradeChain)
				break
			}
			to = next.To
			// Extend the chain using next pair.
			upgradeChain = append(upgradeChain, next)
			// Upgrade pair is used up, delete it from map.
			delete(upgradePairs, next)
		}
	}

	// Sort the chains by last version.
	slices.SortFunc(upgradeChains, func(a, b []upgradePair) int {
		lastVersionA := a[len(a)-1].To
		lastVersionB := b[len(b)-1].To
		return lastVersionA.Compare(lastVersionB)
	})

	// Convert upgrade chain into contiguous upgrade path string.
	upgradePaths := make([]string, 0)
	for _, chain := range upgradeChains {
		var sb strings.Builder
		for i, pair := range chain {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("%s -> %s", pair.From.String(), pair.To.String()))
			} else {
				sb.WriteString(fmt.Sprintf(" -> %s", pair.To.String()))
			}
		}
		upgradePaths = append(upgradePaths, sb.String())
	}

	return upgradePaths
}

func findNextPath(to ech.Version, upgradePairs map[upgradePair]struct{}) (upgradePair, bool) {
	for next := range upgradePairs {
		if to == next.From {
			return next, true
		}
	}
	return upgradePair{}, false
}

// choose returns n elements randomly chosen from the list.
func choose[T any](list []T, n int) []T {
	if n < 0 {
		panic("choose: n cannot be negative")
	}

	if n == 1 {
		return []T{list[rand.IntN(len(list))]}
	}

	if n > len(list) {
		n = len(list)
	}
	indices := rand.Perm(len(list))
	result := make([]T, 0, n)
	for i, idx := range indices {
		if i >= n {
			break
		}
		result = append(result, list[idx])
	}
	return result
}
