package functionaltests

import (
	"testing"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

func TestUpgrade_9_0_to_9_1_Snapshot(t *testing.T) {
	t.Parallel()

	runBasicUpgradeTest(
		t,
		basicUpgradeVersionConfig{
			version:         getLatestSnapshot(t, "9.0"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		basicUpgradeVersionConfig{
			version:         getLatestSnapshot(t, "9.1"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	)
}

func TestUpgrade_9_0_to_9_1_BC(t *testing.T) {
	t.Parallel()

	runBasicUpgradeTest(
		t,
		basicUpgradeVersionConfig{
			version:         getLatestVersion(t, "9.0"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		basicUpgradeVersionConfig{
			version:         getBCVersionOrSkip(t, "9.1"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	)
}

func TestUpgrade_8_19_to_9_1_Snapshot(t *testing.T) {
	t.Parallel()

	runBasicUpgradeTest(
		t,
		basicUpgradeVersionConfig{
			version:         getLatestSnapshot(t, "8.19"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		basicUpgradeVersionConfig{
			version:         getLatestSnapshot(t, "9.1"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	)
}

func TestUpgrade_8_19_to_9_1_BC(t *testing.T) {
	t.Parallel()

	runBasicUpgradeTest(
		t,
		basicUpgradeVersionConfig{
			version:         getLatestVersion(t, "8.19"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		basicUpgradeVersionConfig{
			version:         getBCVersionOrSkip(t, "9.1"),
			preferILM:       true,
			indexManagement: managedByILM,
		},
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	)
}
