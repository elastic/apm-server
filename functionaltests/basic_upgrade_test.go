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

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/asserts"
	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

// runBasicUpgradeILMTest performs a basic upgrade test from `fromVersion` to
// `toVersion`. The test assumes that all data streams are using Index
// Lifecycle Management (ILM) instead of Data Stream Lifecycle Management
// (DSL), which should be the case for most recent APM data streams.
func runBasicUpgradeILMTest(
	t *testing.T,
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored []types.Query,
) {
	// All data streams in this upgrade should be ILM, with no rollover.
	checkILM := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	runAllBasicUpgradeScenarios(
		t,
		fromVersion, toVersion,
		checkILM, checkILM, checkILM,
		apmErrorLogsIgnored,
	)
}

// runBasicUpgradeLazyRolloverDSLTest performs a basic upgrade test from
// `fromVersion` to `toVersion`. The test assumes that all data streams are
// using Data Stream Lifecycle Management (DSL) instead of Index Lifecycle
// Management (ILM).
//
// In 8.15, the data stream management was migrated from ILM to DSL.
// However, a bug was introduced, causing data streams to be unmanaged.
// See https://github.com/elastic/apm-server/issues/13898.
//
// It was fixed by defaulting data stream management to DSL, and eventually
// reverted back to ILM in 8.17. Therefore, data streams created in 8.15 and
// 8.16 are managed by DSL instead of ILM.
func runBasicUpgradeLazyRolloverDSLTest(
	t *testing.T,
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored []types.Query,
) {
	checkDSL := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      managedByDSL,
		IndicesPerDS:     1,
		IndicesManagedBy: []string{managedByDSL},
	}
	// Verify lazy rollover happened, i.e. 2 indices per data stream.
	checkDSLRollover := asserts.CheckDataStreamsWant{
		Quantity:         8,
		PreferIlm:        false,
		DSManagedBy:      managedByDSL,
		IndicesPerDS:     2,
		IndicesManagedBy: []string{managedByDSL, managedByDSL},
	}

	runAllBasicUpgradeScenarios(
		t,
		fromVersion, toVersion,
		checkDSL, checkDSL, checkDSLRollover,
		apmErrorLogsIgnored,
	)
}

func runAllBasicUpgradeScenarios(
	t *testing.T,
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	checkPreUpgradeAfterIngest asserts.CheckDataStreamsWant,
	checkPostUpgradeBeforeIngest asserts.CheckDataStreamsWant,
	checkPostUpgradeAfterIngest asserts.CheckDataStreamsWant,
	apmErrorLogsIgnored []types.Query,
) {
	t.Run("Default", func(t *testing.T) {
		t.Parallel()

		r := testStepsRunner{
			Steps: []testStep{
				createStep{DeployVersion: fromVersion},
				ingestionStep{CheckDataStream: checkPreUpgradeAfterIngest},
				upgradeStep{NewVersion: toVersion, CheckDataStream: checkPostUpgradeBeforeIngest},
				ingestionStep{CheckDataStream: checkPostUpgradeAfterIngest},
				checkErrorLogsStep{APMErrorLogsIgnored: apmErrorLogsIgnored},
			},
		}

		r.Run(t)
	})

	t.Run("Reroute", func(t *testing.T) {
		t.Parallel()

		rerouteNamespace := "rerouted"
		setupFn := stepFunc(func(t *testing.T, ctx context.Context, e *testStepEnv, previousRes testStepResult) testStepResult {
			t.Log("create reroute processors")
			createRerouteIngestPipeline(t, ctx, e.esc, rerouteNamespace)
			return previousRes
		})
		r := testStepsRunner{
			DataStreamNamespace: rerouteNamespace,
			Steps: []testStep{
				createStep{DeployVersion: fromVersion},
				customStep{Func: setupFn},
				ingestionStep{CheckDataStream: checkPreUpgradeAfterIngest},
				upgradeStep{NewVersion: toVersion, CheckDataStream: checkPostUpgradeBeforeIngest},
				ingestionStep{CheckDataStream: checkPostUpgradeAfterIngest},
				checkErrorLogsStep{APMErrorLogsIgnored: apmErrorLogsIgnored},
			},
		}

		r.Run(t)
	})
}
