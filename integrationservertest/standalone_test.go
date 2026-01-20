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
	"maps"
	"testing"

	"github.com/elastic/apm-server/integrationservertest/internal/asserts"
	"github.com/elastic/apm-server/integrationservertest/internal/ech"
)

func TestStandaloneManaged_7_17_to_8_x_to_9_x_Snapshot(t *testing.T) {
	from8 := getLatestSnapshot(t, "8")
	to9 := getLatestSnapshot(t, "9")
	if !vsCache.CanUpgrade(from8, to9) {
		t.Fatalf("upgrade from %s to %s is not allowed", from8, to9)
		return
	}

	config, err := parseConfigFile(upgradeConfigFilename)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Managed8", func(t *testing.T) {
		t.Parallel()
		runner := managed8Runner(from8, to9, config)
		runner.Run(t)
	})

	t.Run("Managed9", func(t *testing.T) {
		t.Parallel()
		runner := managed9Runner(from8, to9, config)
		runner.Run(t)
	})
}

var (
	expectILM = asserts.DataStreamExpectation{
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	}
)

func expectationsFor9x(
	version8 ech.Version,
	version9 ech.Version,
	expect8 map[string]asserts.DataStreamExpectation,
	config upgradeTestConfig,
) map[string]asserts.DataStreamExpectation {
	expect9 := maps.Clone(expect8)
	if config.LazyRollover(version8, version9) {
		for k, v := range expect9 {
			expect9[k] = asserts.DataStreamExpectation{
				PreferIlm:        v.PreferIlm,
				DSManagedBy:      v.DSManagedBy,
				IndicesManagedBy: append(v.IndicesManagedBy, managedByILM),
			}
		}
	}
	return expect9
}

func managed8Runner(fromVersion8, toVersion9 ech.Version, config upgradeTestConfig) testStepsRunner {
	expect8 := dataStreamsExpectations(expectILM)
	expect9 := expectationsFor9x(fromVersion8, toVersion9, expect8, config)

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
			// Start from 8.x.
			createStep{
				DeployVersion:     fromVersion8,
				APMDeploymentMode: apmStandalone,
				CleanupOnFailure:  *cleanupOnFailure,
			},
			ingestStep{CheckDataStreams: expect8},
			// Migrate to managed.
			migrateManagedStep{},
			ingestStep{CheckDataStreams: expect8},
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
				},
				APMErrorLogsIgnored: apmErrorLogs{
					bulkIndexingFailed,
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache403,
					refreshCache503,
					refreshCacheCtxDeadline,
					refreshCacheESConfigInvalid,
					populateSourcemapFetcher403,
					syncSourcemapFetcher403,
				},
			},
		},
	}
}

func managed9Runner(fromVersion8, toVersion9 ech.Version, config upgradeTestConfig) testStepsRunner {
	expect8 := dataStreamsExpectations(expectILM)
	expect9 := expectationsFor9x(fromVersion8, toVersion9, expect8, config)

	return testStepsRunner{
		Target: *target,
		Steps: []testStep{
			// Start from 8.x.
			createStep{
				DeployVersion:     fromVersion8,
				APMDeploymentMode: apmStandalone,
				CleanupOnFailure:  *cleanupOnFailure,
			},
			ingestStep{CheckDataStreams: expect8},
			// Resolve deprecations and upgrade to 9.x.
			resolveDeprecationsStep{},
			upgradeStep{
				NewVersion:       toVersion9,
				CheckDataStreams: expect8,
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
					bulkIndexingFailed,
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache503,
					refreshCache403,
					refreshCacheCtxDeadline,
					refreshCacheESConfigInvalid,
					populateSourcemapFetcher403,
					syncSourcemapFetcher403,
				},
			},
		},
	}
}
