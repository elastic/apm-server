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

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
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
	testCase := singleUpgradeTestCase{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		apmErrorLogsIgnored: apmErrorLogsIgnored,
	}

	runAllBasicUpgradeScenarios(t, testCase)
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
	testCase := singleUpgradeTestCase{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Verify lazy rollover happened, i.e. 2 indices per data stream.
		// Check data streams are managed by DSL.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByDSL, managedByDSL},
		},
		apmErrorLogsIgnored: apmErrorLogsIgnored,
	}

	runAllBasicUpgradeScenarios(t, testCase)
}

func runAllBasicUpgradeScenarios(t *testing.T, testCase singleUpgradeTestCase) {
	t.Run("Default", func(t *testing.T) {
		t.Parallel()
		tt := testCase
		tt.Run(t)
	})

	t.Run("Reroute", func(t *testing.T) {
		t.Parallel()
		tt := testCase
		rerouteNamespace := "rerouted"
		tt.dataStreamNamespace = rerouteNamespace
		tt.setupFn = func(t *testing.T, ctx context.Context, esc *esclient.Client, kbc *kbclient.Client) error {
			t.Log("create reroute processors")
			return createRerouteIngestPipeline(t, ctx, esc, rerouteNamespace)
		}
		tt.Run(t)
	})
}
