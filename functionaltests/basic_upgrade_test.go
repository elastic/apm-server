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

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/elastic/apm-server/functionaltests/internal/ecclient"
)

type basicUpgradeVersionConfig struct {
	version         ecclient.StackVersion
	preferILM       bool
	indexManagement string
}

// testBasicUpgradeILM performs a basic upgrade test from `fromVersion`
// to `toVersion`. This test assumes that all data streams and indices
// (before and after upgrade) are managed exclusively by ILM.
func testBasicUpgradeILM(
	t *testing.T,
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored []types.Query,
) {
	testCase := singleUpgradeTestCase{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		checkPreUpgradeAfterIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeBeforeIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeAfterIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		apmErrorLogsIgnored: apmErrorLogsIgnored,
	}

	runBasicUpgradeTestScenarios(t, testCase)
}

// testBasicUpgradeILMLazyRollover performs a basic upgrade test from
// `fromVersion` to `toVersion`. This test assumes that all data streams
// and indices (before and after upgrade) are managed exclusively by ILM.
//
// It also verifies that lazy rollover happened after post-upgrade
// ingestion.
func testBasicUpgradeILMLazyRollover(
	t *testing.T,
	fromVersion ecclient.StackVersion,
	toVersion ecclient.StackVersion,
	apmErrorLogsIgnored []types.Query,
) {
	testCase := singleUpgradeTestCase{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		checkPreUpgradeAfterIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeBeforeIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		// Verify lazy rollover happened, i.e. 2 indices per data stream.
		checkPostUpgradeAfterIngest: checkDataStreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByILM, managedByILM},
		},
		apmErrorLogsIgnored: apmErrorLogsIgnored,
	}

	runBasicUpgradeTestScenarios(t, testCase)
}

func runBasicUpgradeTestScenarios(t *testing.T, testCase singleUpgradeTestCase) {
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
