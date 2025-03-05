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
)

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
	t.Parallel()
	ecAPICheck(t)

	tt := singleUpgradeTestCase{
		fromVersion: "8.15.4",
		toVersion:   "8.16.0",
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
		// Check data streams are managed by DSL since they are created after 8.15.0.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByDSL,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByDSL, managedByDSL},
		},
	}

	tt.Run(t)
}

func TestUpgrade_8_13_4_to_8_16_0_Reroute(t *testing.T) {
	t.Parallel()
	ecAPICheck(t)

	tt := singleUpgradeTestCase{
		fromVersion: "8.13.4",
		toVersion:   "8.16.0",
		preUpgradeBeforeIngestSetup: func(t *testing.T, ctx context.Context, cfg *config, deps dependencies) bool {
			t.Log("create reroute processors")
			rerouteNamespace := "rerouted"
			cfg.DSNamespace = rerouteNamespace
			require.NoError(t, deps.ESClient.CreateRerouteProcessors(ctx, rerouteNamespace))
			return true
		},
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		postUpgradeBeforeIngestSetup: func(t *testing.T, ctx context.Context, cfg *config, deps dependencies) bool {
			t.Log("perform manual rollovers")
			require.NoError(t, deps.ESClient.PerformManualRollovers(ctx, cfg.DSNamespace))
			return true
		},
		// Verify manual rollover happened, i.e. 2 indices per data stream.
		// Check data streams are managed by ILM since they are created before 8.15.0.
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByILM, managedByILM},
		},
	}

	tt.Run(t)
}
