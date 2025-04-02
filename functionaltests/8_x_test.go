package functionaltests

import (
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
)

func TestUpgrade_7_17_to_8_x_Snapshot_Standalone_to_Managed(t *testing.T) {
	fromVersion := getLatestSnapshot(t, "7.17")
	toVersion := getLatestSnapshot(t, "8")

	// Data streams in 8.x should be all ILM.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	runner := testStepsRunner{
		Steps: []testStep{
			createStep{
				DeployVersion:     fromVersion,
				APMDeploymentMode: apmStandalone,
			},
			ingestLegacyStep{},
			upgradeLegacyStep{NewVersion: toVersion},
			ingestStep{CheckDataStream: checkILM},
			migrateManagedStep{},
			ingestStep{CheckDataStream: checkILM},
			checkErrorLogsStep{
				ESErrorLogsIgnored: esErrorLogs{
					eventLoopShutdown,
				},
				APMErrorLogsIgnored: apmErrorLogs{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache503,
					// TODO: remove once fixed
					populateSourcemapFetcher403,
				},
			},
		},
	}

	runner.Run(t)
}

func TestUpgrade_7_17_to_8_x_BC_Standalone_to_Managed(t *testing.T) {
	fromVersion := getLatestVersion(t, "7.17")
	toVersion := getBCVersionOrSkip(t, "8")

	// Data streams in 8.x should be all ILM.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	runner := testStepsRunner{
		Steps: []testStep{
			createStep{
				DeployVersion:     fromVersion,
				APMDeploymentMode: apmStandalone,
			},
			ingestLegacyStep{},
			upgradeLegacyStep{NewVersion: toVersion},
			ingestStep{CheckDataStream: checkILM},
			migrateManagedStep{},
			ingestStep{CheckDataStream: checkILM},
			checkErrorLogsStep{
				ESErrorLogsIgnored: esErrorLogs{
					eventLoopShutdown,
				},
				APMErrorLogsIgnored: apmErrorLogs{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache503,
					// TODO: remove once fixed
					populateSourcemapFetcher403,
				},
			},
		},
	}

	runner.Run(t)
}
