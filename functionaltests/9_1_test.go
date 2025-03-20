package functionaltests

import "testing"

func TestUpgrade_8_19_to_9_1(t *testing.T) {
	t.Parallel()
	runBasicUpgradeSnapshotTest(t, "8.19", "9.1")
}
