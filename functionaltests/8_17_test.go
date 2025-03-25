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

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// In 8.15, the data stream management was migrated from ILM to DSL.
// However, a bug was introduced, causing data streams to be unmanaged.
// See https://github.com/elastic/apm-server/issues/13898.
//
// It was fixed by defaulting data stream management to DSL, and eventually
// reverted back to ILM in 8.17. Therefore, data streams created in 8.16
// are managed by DSL, while data streams created in 8.17 are managed by ILM.

func TestUpgrade_8_16_to_8_17_Snapshot(t *testing.T) {
	t.Parallel()

	testCase := singleUpgradeTestCase{
		fromVersion: getLatestSnapshot(t, "8.16"),
		toVersion:   getLatestSnapshot(t, "8.17"),
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Old index still managed by DSL, despite data stream being
		// managed by ILM.
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Verify lazy rollover happened, i.e. 2 indices per data stream.
		// New index managed by ILM but old index managed by DSL.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByDSL, managedByILM},
		},
		apmErrorLogsIgnored: []types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	}

	runBasicUpgradeTestScenarios(t, testCase)
}

func TestUpgrade_8_16_to_8_17_BC(t *testing.T) {
	t.Parallel()

	testCase := singleUpgradeTestCase{
		fromVersion: getLatestVersionOrSkip(t, "8.16"),
		toVersion:   getLatestBCOrSkip(t, "8.17"),
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Old index still managed by DSL, despite data stream being
		// managed by ILM.
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Verify lazy rollover happened, i.e. 2 indices per data stream.
		// New index managed by ILM but old index managed by DSL.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByDSL, managedByILM},
		},
		apmErrorLogsIgnored: []types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			populateSourcemapFetcher403,
		},
	}

	runBasicUpgradeTestScenarios(t, testCase)
}
