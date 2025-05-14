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
)

// In 8.15, the data stream management was migrated from ILM to DSL.
// However, a bug was introduced, causing data streams to be unmanaged.
// See https://github.com/elastic/apm-server/issues/13898.
//
// It was fixed by defaulting data stream management to DSL, and eventually
// reverted back to ILM in 8.17. Therefore, data streams created in 8.15 and
// 8.16 are managed by DSL instead of ILM.

func TestUpgrade_8_16_to_8_17_Snapshot(t *testing.T) {
	t.Parallel()
	from := getLatestSnapshot(t, "8.16")
	to := getLatestSnapshot(t, "8.17")
	if !from.CanUpgradeTo(to.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from.Version, to.Version)
		return
	}

	scenarios := allBasicUpgradeScenarios(
		from.Version,
		to.Version,
		// Data streams managed by DSL pre-upgrade.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDS:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Data streams managed by ILM post-upgrade.
		// However, the index created before upgrade is still managed by DSL.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDS:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Verify lazy rollover happened, i.e. 2 indices.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDS:     2,
			IndicesManagedBy: []string{managedByDSL, managedByILM},
		},
		apmErrorLogs{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			refreshCacheCtxCanceled,
			populateSourcemapFetcher403,
		},
	)
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}

func TestUpgrade_8_16_to_8_17_BC(t *testing.T) {
	t.Parallel()
	from := getLatestVersionOrSkip(t, "8.16")
	to := getLatestBCOrSkip(t, "8.17")
	if !from.CanUpgradeTo(to.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from.Version, to.Version)
		return
	}

	scenarios := allBasicUpgradeScenarios(
		from.Version,
		to.Version,
		// Data streams managed by DSL pre-upgrade.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDS:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Data streams managed by ILM post-upgrade.
		// However, the index created before upgrade is still managed by DSL.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDS:     1,
			IndicesManagedBy: []string{managedByDSL},
		},
		// Verify lazy rollover happened, i.e. 2 indices.
		asserts.CheckDataStreamsWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDS:     2,
			IndicesManagedBy: []string{managedByDSL, managedByILM},
		},
		apmErrorLogs{
			tlsHandshakeError,
			esReturnedUnknown503,
			refreshCache503,
			refreshCacheCtxCanceled,
			populateSourcemapFetcher403,
		},
	)
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}
