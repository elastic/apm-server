package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

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
		"Use SNAPSHOT versions instead of released versions / BCs.",
	)
)

const (
	// We will only use versions >= 8.15 in the tests.
	versionCutoffMajor = 8
	versionCutoffMinor = 15
)

func main() {
	flag.Parse()

	// This is a simple check to alert users if this necessary env var
	// is not available.
	//
	// A valid API key is required to query Elastic Cloud APIs.
	ecAPIKey := os.Getenv("EC_API_KEY")
	if ecAPIKey == "" {
		log.Fatal("EC_API_KEY env var not set")
		return
	}

	ctx := context.Background()
	ecRegion := integrationservertest.RegionFrom(*target)
	ecc, err := ech.NewClient(integrationservertest.EndpointFrom(*target), ecAPIKey)
	if err != nil {
		log.Fatal(err)
		return
	}

	vsCache, err := ech.NewVersionsCache(ctx, ecc, ecRegion)
	if err != nil {
		log.Fatal(err)
		return
	}

	versions, err := fetchTestVersions(ctx, vsCache)
	if err != nil {
		log.Fatal(err)
		return
	}

	for _, version := range versions {
		upgradeFromVersions := vsCache.GetUpgradeFromVersions(version).
			Filter(func(v ech.Version) bool {
				return v.IsSnapshot() && versionMeetCutoff(v) && v.MajorMinor() != version.MajorMinor()
			})
		upgradeFromVersions = choose(latestOfEachMajorMinor(upgradeFromVersions), 2)
		upgradeFromVersions.Sort()

		for _, from := range upgradeFromVersions {
			fmt.Println(from.String() + " -> " + version.String())
		}
	}
}

func latestOfEachMajorMinor(versions ech.Versions) ech.Versions {
	var result ech.Versions
	var currMajorMinor string

	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		if version.MajorMinor() != currMajorMinor {
			result = append(result, version)
			currMajorMinor = version.MajorMinor()
		}
	}

	return result
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

func versionMeetCutoff(version ech.Version) bool {
	return version.Major > versionCutoffMajor ||
		(version.Major == versionCutoffMajor && version.Minor >= versionCutoffMinor)
}
