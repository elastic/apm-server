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
)

// In 8.15, the data stream management was migrated from ILM to DSL.
// However, a bug was introduced, causing data streams to be unmanaged.
// See https://github.com/elastic/apm-server/issues/13898.
//
// It was fixed by defaulting data stream management to DSL, and eventually
// reverted back to ILM in 8.17. Therefore, data streams created in 8.15 and
// 8.16 are managed by DSL instead of ILM.

func TestUpgrade_8_15_to_8_16__Snapshot(t *testing.T) {
	t.Parallel()

	scenarios := basicUpgradeLazyRolloverDSLTestScenarios(
		getLatestSnapshot(t, "8.15"),
		getLatestSnapshot(t, "8.16"),
		apmErrorLogs{
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
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}

func TestUpgrade_8_15_to_8_16__BC(t *testing.T) {
	t.Parallel()

	scenarios := basicUpgradeLazyRolloverDSLTestScenarios(
		getLatestVersionOrSkip(t, "8.15"),
		getLatestBCOrSkip(t, "8.16"),
		apmErrorLogs{
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
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}
