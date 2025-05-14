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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

type basicUpgradeTestScenario struct {
	Name   string
	Runner testStepsRunner
}

// basicUpgradeILMTestScenarios returns all scenarios for basic upgrade test
// from `fromVersion` to `toVersion`. The test assumes that all data streams
// (before and after upgrade) are using Index Lifecycle Management (ILM)
// instead of Data Stream Lifecycle Management (DSL), which should be the case
// for most recent APM data streams.
func basicUpgradeILMTestScenarios(
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored apmErrorLogs,
) []basicUpgradeTestScenario {
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	}

	return allBasicUpgradeScenarios(
		fromVersion, toVersion,
		checkILM, checkILM, checkILM,
		apmErrorLogsIgnored,
	)
}

// basicUpgradeLazyRolloverILMTestScenarios returns all scenarios for basic
// upgrade test from `fromVersion` to `toVersion`. The test assumes that all
// data streams (before and after upgrade) are using Index Lifecycle Management
// (ILM) instead of Data Stream Lifecycle Management (DSL), which should be the
// case for most recent APM data streams. It will also verify that lazy
// rollover happened on post-upgrade ingestion.
func basicUpgradeLazyRolloverILMTestScenarios(
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored apmErrorLogs,
) []basicUpgradeTestScenario {
	// All data streams should be managed by ILM.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM},
	}
	// Verify lazy rollover happened, i.e. 2 indices per data stream.
	checkILMRollover := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesManagedBy: []string{managedByILM, managedByILM},
	}

	return allBasicUpgradeScenarios(
		fromVersion, toVersion,
		checkILM, checkILM, checkILMRollover,
		apmErrorLogsIgnored,
	)
}

// basicUpgradeLazyRolloverDSLTestScenarios returns all scenarios for basic
// upgrade test from `fromVersion` to `toVersion`. The test assumes that all
// data streams (before and after upgrade) are using Data Stream Lifecycle
// Management (DSL) instead of Index Lifecycle Management (ILM). It will also
// verify that lazy rollover happened on post-upgrade ingestion.
func basicUpgradeLazyRolloverDSLTestScenarios(
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored apmErrorLogs,
) []basicUpgradeTestScenario {
	// All data streams should be managed by DSL.
	checkDSL := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      managedByDSL,
		IndicesManagedBy: []string{managedByDSL},
	}
	// Verify lazy rollover happened, i.e. 2 indices per data stream.
	checkDSLRollover := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      managedByDSL,
		IndicesManagedBy: []string{managedByDSL, managedByDSL},
	}

	return allBasicUpgradeScenarios(
		fromVersion, toVersion,
		checkDSL, checkDSL, checkDSLRollover,
		apmErrorLogsIgnored,
	)
}

// allBasicUpgradeScenarios returns all basic upgrade test scenarios.
// The scenarios involved are:
//
//   - Default: The cluster is created, some data is ingested and the first
//     check ensures that it's in the expected state. Then, an upgrade
//     is triggered, and a second check confirms that the state did not
//     drift after upgrade. A new ingestion is performed, and a third
//     check verifies that ingestion works as expected after upgrade.
//     Finally, error logs are examined to ensure there are no unexpected
//     errors.
//
//   - Reroute: Same as Default scenario, except after the cluster is created,
//     we insert a reroute ingest pipeline to reroute all APM data streams to
//     a new namespace. This test is to ensure that APM data streams rerouting
//     still works as expected across ingestion and upgrade.
//     See https://github.com/elastic/apm-server/issues/14060 for motivation.
func allBasicUpgradeScenarios(
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	checkPreUpgradeAfterIngest asserts.CheckDataStreamsWant,
	checkPostUpgradeBeforeIngest asserts.CheckDataStreamsWant,
	checkPostUpgradeAfterIngest asserts.CheckDataStreamsWant,
	apmErrorLogsIgnored apmErrorLogs,
) []basicUpgradeTestScenario {
	var scenarios []basicUpgradeTestScenario

	// Default
	scenarios = append(scenarios, basicUpgradeTestScenario{
		Name: "Default",
		Runner: testStepsRunner{
			Steps: []testStep{
				createStep{DeployVersion: fromVersion},
				ingestStep{CheckDataStream: checkPreUpgradeAfterIngest},
				upgradeStep{NewVersion: toVersion, CheckDataStream: checkPostUpgradeBeforeIngest},
				ingestStep{CheckDataStream: checkPostUpgradeAfterIngest},
				checkErrorLogsStep{APMErrorLogsIgnored: apmErrorLogsIgnored},
			},
		},
	})

	// Reroute
	rerouteNamespace := "rerouted"
	setupFn := stepFunc(func(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
		t.Log("create reroute processors")
		createRerouteIngestPipeline(t, ctx, e.esc, rerouteNamespace)
		return previousRes
	})
	scenarios = append(scenarios, basicUpgradeTestScenario{
		Name: "Reroute",
		Runner: testStepsRunner{
			DataStreamNamespace: rerouteNamespace,
			Steps: []testStep{
				createStep{DeployVersion: fromVersion},
				customStep{Func: setupFn},
				ingestStep{CheckDataStream: checkPreUpgradeAfterIngest},
				upgradeStep{NewVersion: toVersion, CheckDataStream: checkPostUpgradeBeforeIngest},
				ingestStep{CheckDataStream: checkPostUpgradeAfterIngest},
				checkErrorLogsStep{APMErrorLogsIgnored: apmErrorLogsIgnored},
			},
		},
	})

	return scenarios
}

// createRerouteIngestPipeline creates custom pipelines to reroute logs, metrics and traces to different
// data streams specified by namespace.
func createRerouteIngestPipeline(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) {
	t.Helper()
	for _, pipeline := range []string{"logs@custom", "metrics@custom", "traces@custom"} {
		err := esc.CreateIngestPipeline(ctx, pipeline, []types.ProcessorContainer{
			{
				Reroute: &types.RerouteProcessor{
					Namespace: []string{namespace},
				},
			},
		})
		require.NoError(t, err)
	}
}

// performManualRollovers rollover all logs, metrics and traces data streams to new indices.
func performManualRollovers(t *testing.T, ctx context.Context, esc *esclient.Client, namespace string) {
	t.Helper()

	for _, ds := range allDataStreams(namespace) {
		err := esc.PerformManualRollover(ctx, ds)
		require.NoError(t, err)
	}
}
