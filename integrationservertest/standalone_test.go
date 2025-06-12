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

package integrationservertest

import (
	"testing"

	"github.com/elastic/apm-server/integrationservertest/internal/asserts"
	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

func TestStandaloneManaged_7_17_to_8_x_to_9_x_Snapshot(t *testing.T) {
	from7 := vsCache.GetLatestSnapshot(t, "7.17")
	to8 := vsCache.GetLatestSnapshot(t, "8")
	to9 := vsCache.GetLatestSnapshot(t, "9")
	if !vsCache.CanUpgradeTo(from7, to8) {
		t.Fatalf("upgrade from %s to %s is not allowed", from7, to8)
		return
	}
	if !vsCache.CanUpgradeTo(to8, to9) {
		t.Fatalf("upgrade from %s to %s is not allowed", to8, to9)
		return
	}

	t.Run("Managed7", func(t *testing.T) {
		t.Parallel()
		runner := managed7Runner(from7, to8, to9)
		runner.Run(t)
	})

	t.Run("Managed8", func(t *testing.T) {
		t.Parallel()
		runner := managed8Runner(from7, to8, to9)
		runner.Run(t)
	})

	t.Run("Managed9", func(t *testing.T) {
		t.Parallel()
		runner := managed9Runner(from7, to8, to9)
		runner.Run(t)
	})
}

func managed7Runner(fromVersion7, toVersion8, toVersion9 ech.Version) testStepsRunner {
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
	expectILMRolloverTwice := asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM, managedByILM, managedByILM},
	}

	expect8 := map[string]asserts.DataStreamExpectation{
		// These data streams are created in 7.x as well, so when we ingest
		// again in 8.x, they will be rolled-over.
		"traces-apm-%s":                     expectILMRollover,
		"metrics-apm.app.opbeans_python-%s": expectILMRollover,
		"metrics-apm.internal-%s":           expectILMRollover,
		"logs-apm.error-%s":                 expectILMRollover,
		// These data streams are only created in 8.x, so no rollover.
		"metrics-apm.service_destination.1m-%s": expectILM,
		"metrics-apm.service_transaction.1m-%s": expectILM,
		"metrics-apm.service_summary.1m-%s":     expectILM,
		"metrics-apm.transaction.1m-%s":         expectILM,
	}

	expect9 := map[string]asserts.DataStreamExpectation{
		// These data streams are rolled-over in 8.x as well, so when we ingest
		// again in 9.x, they will be rolled-over again.
		"traces-apm-%s":                     expectILMRolloverTwice,
		"metrics-apm.app.opbeans_python-%s": expectILMRolloverTwice,
		"metrics-apm.internal-%s":           expectILMRolloverTwice,
		"logs-apm.error-%s":                 expectILMRolloverTwice,
		// These data streams are only created in 8.x, so they will only
		// roll over once.
		"metrics-apm.service_destination.1m-%s": expectILMRollover,
		"metrics-apm.service_transaction.1m-%s": expectILMRollover,
		"metrics-apm.service_summary.1m-%s":     expectILMRollover,
		"metrics-apm.transaction.1m-%s":         expectILMRollover,
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
				CheckDataStreams:  expect8,
			},
			// Resolve deprecations and upgrade to 9.x.
			resolveDeprecationsStep{},
			upgradeStep{
				NewVersion:        toVersion9,
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect8,
			},
			ingestStep{
				IgnoreDataStreams: ignoredDataStreams,
				CheckDataStreams:  expect9,
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

func managed8Runner(fromVersion7, toVersion8, toVersion9 ech.Version) testStepsRunner {
	expect := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	})
	expect9 := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM, managedByILM},
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
				CheckDataStreams:  expect9,
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

func managed9Runner(fromVersion7, toVersion8, toVersion9 ech.Version) testStepsRunner {
	expect := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	})
	expect9 := dataStreamsExpectations(asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM, managedByILM},
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
			ingestStep{CheckDataStreams: expect9},
			// Migrate to managed.
			migrateManagedStep{},
			ingestStep{CheckDataStreams: expect9},
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
