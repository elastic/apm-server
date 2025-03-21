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

// runBasicUpgradeTest performs a basic upgrade test from `from.version` to
// `to.version`.
func runBasicUpgradeTest(
	t *testing.T,
	from basicUpgradeVersionConfig,
	to basicUpgradeVersionConfig,
	apmErrorLogsIgnored []types.Query,
) {
	testCase := singleUpgradeTestCase{
		fromVersion: from.version,
		toVersion:   to.version,
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        from.preferILM,
			DSManagedBy:      from.indexManagement,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{from.indexManagement},
		},
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        to.preferILM,
			DSManagedBy:      to.indexManagement,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{to.indexManagement},
		},
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        to.preferILM,
			DSManagedBy:      to.indexManagement,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{to.indexManagement},
		},
		apmErrorLogsIgnored: apmErrorLogsIgnored,
	}

	runAllBasicUpgradeScenarios(t, testCase)
}

// runBasicUpgradeLazyRolloverTest performs a basic upgrade test from
// `from.version` to `to.version`.
func runBasicUpgradeLazyRolloverTest(
	t *testing.T,
	from basicUpgradeVersionConfig,
	to basicUpgradeVersionConfig,
	apmErrorLogsIgnored []types.Query,
) {
	testCase := singleUpgradeTestCase{
		fromVersion: from.version,
		toVersion:   to.version,
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        from.preferILM,
			DSManagedBy:      from.indexManagement,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{from.indexManagement},
		},
		// Old index should still be managed by `from.indexManagement`.
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        to.preferILM,
			DSManagedBy:      to.indexManagement,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{from.indexManagement},
		},
		// Verify lazy rollover happened, i.e. 2 indices per data stream.
		// Old index should be managed by `from.indexManagement` while new
		// index is managed by `to.indexManagement`.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        to.preferILM,
			DSManagedBy:      to.indexManagement,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{from.indexManagement, to.indexManagement},
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
