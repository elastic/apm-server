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

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

func TestUpgrade_8_15_to_8_16_Snapshot(t *testing.T) {
	t.Parallel()

	runBasicUpgradeLazyRolloverDSLTest(
		t,
		getLatestSnapshot(t, "8.15"),
		getLatestSnapshot(t, "8.16"),
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			preconditionFailed,
			populateSourcemapServerShuttingDown,
			refreshCacheCtxDeadline,
			refreshCacheCtxCanceled,
			// TODO: remove once fixed
			populateSourcemapFetcher403,
		},
	)
}

func TestUpgrade_8_15_to_8_16_BC(t *testing.T) {
	t.Parallel()

	runBasicUpgradeLazyRolloverDSLTest(
		t,
		getLatestVersion(t, "8.15"),
		getBCVersionOrSkip(t, "8.16"),
		[]types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			preconditionFailed,
			populateSourcemapServerShuttingDown,
			refreshCacheCtxDeadline,
			refreshCacheCtxCanceled,
			// TODO: remove once fixed
			populateSourcemapFetcher403,
		},
	)
}

func TestUpgrade_8_14_to_8_16_Reroute(t *testing.T) {
	t.Parallel()
	toVersion := getLatestSnapshot(t, "8.16")
	skipNonActiveVersion(t, toVersion)

	rerouteNamespace := "rerouted"
	tt := singleUpgradeTestCase{
		fromVersion:         getLatestSnapshot(t, "8.14"),
		toVersion:           toVersion,
		dataStreamNamespace: rerouteNamespace,
		setupFn: func(t *testing.T, ctx context.Context, esc *esclient.Client, _ *kbclient.Client) error {
			t.Log("create reroute processors")
			return createRerouteIngestPipeline(t, ctx, esc, rerouteNamespace)
		},
		checkPreUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        true,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     1,
			IndicesManagedBy: []string{managedByILM},
		},
		postUpgradeFn: func(t *testing.T, ctx context.Context, esc *esclient.Client, _ *kbclient.Client) error {
			t.Log("perform manual rollovers")
			return performManualRollovers(t, ctx, esc, rerouteNamespace)
		},
		// Verify manual rollover happened, i.e. 2 indices per data stream.
		// Check data streams are managed by ILM since they are created before 8.15.0.
		checkPostUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByILM, managedByILM},
		},
		checkPostUpgradeAfterIngest: checkDatastreamWant{
			Quantity:         8,
			PreferIlm:        false,
			DSManagedBy:      managedByILM,
			IndicesPerDs:     2,
			IndicesManagedBy: []string{managedByILM, managedByILM},
		},

		apmErrorLogsIgnored: []types.Query{
			tlsHandshakeError,
			esReturnedUnknown503,
			preconditionFailed,
			populateSourcemapServerShuttingDown,
			refreshCacheCtxDeadline,
			// TODO: remove once fixed
			populateSourcemapFetcher403,
		},
	}

	tt.Run(t)
}
