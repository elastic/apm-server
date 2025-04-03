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

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

func TestUpgrade_7_17_to_8_x_Snapshot_Standalone_to_Managed(t *testing.T) {
	fromVersion := getLatestSnapshot(t, "7.17")
	toVersion := getLatestSnapshot(t, "8")

	t.Run("UpgradeFirst", func(t *testing.T) {
		t.Parallel()
		runner := upgradeThenManagedRunner(fromVersion, toVersion)
		runner.Run(t)
	})

	t.Run("ManagedFirst", func(t *testing.T) {
		t.Parallel()
		runner := managedThenUpgradeRunner(fromVersion, toVersion)
		runner.Run(t)
	})
}

func TestUpgrade_7_17_to_8_x_BC_Standalone_to_Managed(t *testing.T) {
	fromVersion := getLatestVersion(t, "7.17")
	toVersion := getBCVersionOrSkip(t, "8")

	t.Run("UpgradeFirst", func(t *testing.T) {
		t.Parallel()
		runner := upgradeThenManagedRunner(fromVersion, toVersion)
		runner.Run(t)
	})

	t.Run("ManagedFirst", func(t *testing.T) {
		t.Parallel()
		runner := managedThenUpgradeRunner(fromVersion, toVersion)
		runner.Run(t)
	})
}

func upgradeThenManagedRunner(fromVersion, toVersion ecclient.StackVersion) testStepsRunner {
	// Data streams in 8.x should be all ILM if upgraded to a stack < 8.15 and > 8.16.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	return testStepsRunner{
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
}

func managedThenUpgradeRunner(fromVersion, toVersion ecclient.StackVersion) testStepsRunner {
	// Data streams in 8.x should be all ILM.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	return testStepsRunner{
		Steps: []testStep{
			createStep{
				DeployVersion:     fromVersion,
				APMDeploymentMode: apmStandalone,
			},
			ingestLegacyStep{},
			migrateManagedStep{},
			ingestLegacyStep{},
			upgradeLegacyStep{NewVersion: toVersion},
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
}
