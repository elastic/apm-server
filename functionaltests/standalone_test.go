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

func TestStandaloneManaged_7_17_to_8_x_to_9_x_Snapshot(t *testing.T) {
	from7 := vsCache.GetLatestSnapshot(t, "7.17")
	to8 := vsCache.GetLatestSnapshot(t, "8")
	to9 := vsCache.GetLatestSnapshot(t, "9")
	if !from7.CanUpgradeTo(to8.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from7.Version, to8.Version)
		return
	}
	if !to8.CanUpgradeTo(to9.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", to8.Version, to9.Version)
		return
	}

	t.Run("Managed7", func(t *testing.T) {
		t.Parallel()
		runner := managed7Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed8", func(t *testing.T) {
		t.Parallel()
		runner := managed8Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})

	t.Run("Managed9", func(t *testing.T) {
		t.Parallel()
		runner := managed9Runner(from7.Version, to8.Version, to9.Version)
		runner.Run(t)
	})
}

func managed7Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) testStepsRunner {
	expectILM := asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	}
	expectILMRollover := asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM, managedByILM},
	}

	expect := map[string]asserts.DataStreamExpectation{
		// These data streams are created in 7.x as well, so when we ingest
		// again in 8.x, they will be rolled-over.
		"traces-apm-%s":                     expectILMRollover,
		"metrics-apm.app.opbeans_python-%s": expectILMRollover,
		"metrics-apm.internal-%s":           expectILMRollover,
		"logs-apm.error-%s":                 expectILMRollover,
		// These data streams are only created in 8.x, so they will only have
		// 1 index per.
		"metrics-apm.service_destination.1m-%s": expectILM,
		"metrics-apm.service_transaction.1m-%s": expectILM,
		"metrics-apm.service_summary.1m-%s":     expectILM,
		"metrics-apm.transaction.1m-%s":         expectILM,
	}

	// These data streams are created in 7.x, but not used in 8.x and 9.x,
	// so we ignore them to avoid wrong assertions.
	ignoredDataStreams := []string{
		"metrics-apm.app.opbeans_node-%s",
		"metrics-apm.app.opbeans_ruby-%s",
		"metrics-apm.app.opbeans_go-%s",
	}

	return testStepsRunner{
		Target: *target,
		Steps: []testStep{
			// Start from 7.x.
			createStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: apmStandalone,
			},
			ingestV7Step{},
			// Migrate to managed.
			migrateManagedStep{},
			ingestV7Step{},
			// Upgrade to 8.x.
			upgradeV7Step{NewVersion: toVersion8},
			ingestStep{
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect,
			},
			// Resolve deprecations and upgrade to 9.x.
			resolveDeprecationsStep{},
			upgradeStep{
				NewVersion:        toVersion9,
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect,
			},
			ingestStep{
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect,
			},
			checkErrorLogsStep{
				ESErrorLogsIgnored: esErrorLogs{
					eventLoopShutdown,
					addIndexTemplateTracesError,
				},
				APMErrorLogsIgnored: apmErrorLogs{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache403,
					refreshCache503,
					refreshCacheCtxDeadline,
					refreshCacheESConfigInvalid,
					preconditionClusterInfoCtxCanceled,
					waitServerReadyCtxCanceled,
					grpcServerStopped,
					populateSourcemapFetcher403,
				},
			},
		},
	}
}

func managed8Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) testStepsRunner {
	expect := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	})

	// These data streams are created in 7.x, but not used in 8.x and 9.x,
	// so we ignore them to avoid wrong assertions.
	ignoredDataStreams := []string{
		"metrics-apm.app.opbeans_node-%s",
		"metrics-apm.app.opbeans_ruby-%s",
		"metrics-apm.app.opbeans_go-%s",
	}

	return testStepsRunner{
		Target: *target,
		Steps: []testStep{
			// Start from 7.x.
			createStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: apmStandalone,
				CleanupOnFailure:  *cleanupOnFailure,
			},
			ingestV7Step{},
			// Upgrade to 8.x.
			upgradeV7Step{NewVersion: toVersion8},
			ingestStep{CheckDataStreams: expect},
			// Migrate to managed
			migrateManagedStep{},
			ingestStep{CheckDataStreams: expect},
			// Resolve deprecations and upgrade to 9.x.
			resolveDeprecationsStep{},
			upgradeStep{
				NewVersion:        toVersion9,
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect,
			},
			ingestStep{
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect,
			},
			checkErrorLogsStep{
				ESErrorLogsIgnored: esErrorLogs{
					eventLoopShutdown,
				},
				APMErrorLogsIgnored: apmErrorLogs{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache403,
					refreshCache503,
					refreshCacheCtxDeadline,
					refreshCacheESConfigInvalid,
					populateSourcemapFetcher403,
				},
			},
		},
	}
}

func managed9Runner(fromVersion7, toVersion8, toVersion9 ecclient.StackVersion) testStepsRunner {
	// Data streams created in latest 8.x and 9.x should be all ILM.
	expect := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	})

	return testStepsRunner{
		Target: *target,
		Steps: []testStep{
			// Start from 7.x.
			createStep{
				DeployVersion:     fromVersion7,
				APMDeploymentMode: apmStandalone,
				CleanupOnFailure:  *cleanupOnFailure,
			},
			ingestV7Step{},
			// Upgrade to 8.x.
			upgradeV7Step{NewVersion: toVersion8},
			ingestStep{CheckDataStreams: expect},
			// Resolve deprecations and upgrade to 9.x.
			resolveDeprecationsStep{},
			upgradeStep{
				NewVersion:       toVersion9,
				CheckDataStreams: expect,
			},
			ingestStep{CheckDataStreams: expect},
			// Migrate to managed.
			migrateManagedStep{},
			ingestStep{CheckDataStreams: expect},
			checkErrorLogsStep{
				ESErrorLogsIgnored: esErrorLogs{
					eventLoopShutdown,
				},
				APMErrorLogsIgnored: apmErrorLogs{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache503,
					refreshCache403,
					refreshCacheCtxDeadline,
					refreshCacheESConfigInvalid,
					populateSourcemapFetcher403,
				},
			},
		},
	}
}
