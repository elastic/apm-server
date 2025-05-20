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

package deep

import (
	"testing"

	"github.com/elastic/apm-server/functionaltests"
	"github.com/elastic/apm-server/functionaltests/internal/steps"
)

func TestUpgrade_8_17_to_8_18_Snapshot(t *testing.T) {
	t.Parallel()
	from := versionsCache.GetLatestSnapshot(t, "8.17")
	to := versionsCache.GetLatestSnapshot(t, "8.18")
	if !from.CanUpgradeTo(to.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from.Version, to.Version)
		return
	}

	scenarios := deepUpgradeILMTestScenarios(
		from.Version,
		to.Version,
		steps.APMErrorLogs{
			functionaltests.TLSHandshakeError,
			functionaltests.ESReturnedUnknown503,
			functionaltests.RefreshCache503,
			functionaltests.PopulateSourcemapFetcher403,
		},
	)
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}

func TestUpgrade_8_17_to_8_18_BC(t *testing.T) {
	t.Parallel()
	from := versionsCache.GetLatestVersionOrSkip(t, "8.17")
	to := versionsCache.GetLatestBCOrSkip(t, "8.18")
	if !from.CanUpgradeTo(to.Version) {
		t.Skipf("upgrade from %s to %s is not allowed", from.Version, to.Version)
		return
	}

	scenarios := deepUpgradeILMTestScenarios(
		from.Version,
		to.Version,
		steps.APMErrorLogs{
			functionaltests.TLSHandshakeError,
			functionaltests.ESReturnedUnknown503,
			functionaltests.RefreshCache503,
			functionaltests.PopulateSourcemapFetcher403,
		},
	)
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()
			scenario.Runner.Run(t)
		})
	}
}
