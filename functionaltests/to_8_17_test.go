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
	"fmt"
	"testing"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
	"github.com/elastic/apm-server/functionaltests/internal/kbclient"
)

func TestUpgradeTo8170_plain(t *testing.T) {
	t.Parallel()
	ecAPICheck(t)

	runESSUpgradeTest(t, essUpgradeTestCase{
		from: "8.16.1",
		to:   "8.17.0",

		beforeUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    false,
			DSManagedBy:  lifecycleDSL,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL},
			},
		},
		afterUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL},
			},
		},
		afterUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 2,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL, PreferIlm: false},
				{ManagedBy: lifecycleILM, PreferIlm: true},
			},
		},
	})
}

// TestUpgradeTo8170_reroute checks for regressions in upgrades when
// using an ingest pipeline with a reroute processor.
func TestUpgradeTo8170_reroute(t *testing.T) {
	t.Parallel()
	ecAPICheck(t)

	runESSUpgradeTest(t, essUpgradeTestCase{
		from: "8.16.1",
		to:   "8.17.0",

		expectedDsDocCountForAIngestRun: expectedIngestForASingleRun("rerouted"),

		setupFn: func(ecc *esclient.Client, _ *kbclient.Client, _ esclient.Config) error {
			return createRerouteIngestPipelines(t, context.Background(), ecc)
		},

		beforeUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    false,
			DSManagedBy:  lifecycleDSL,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL},
			},
		},
		afterUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL},
			},
		},
		afterUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 2,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleDSL, PreferIlm: false},
				{ManagedBy: lifecycleILM, PreferIlm: true},
			},
		},
	})
}

func TestUpgradeTo8170_withAPMIntegration(t *testing.T) {
	t.Parallel()
	ecAPICheck(t)

	runESSUpgradeTest(t, essUpgradeTestCase{
		from: "8.14.3",
		to:   "8.17.0",

		// APM integration is always enabled through the Elastic Agent Cloud Policy,
		// no further setup is necessary.
		afterUpgrade: func(_ *esclient.Client, kbc *kbclient.Client, _ esclient.Config) error {
			policies, err := kbc.AgentPolicies(context.Background())
			if err != nil {
				return fmt.Errorf("cannot retrieve agent policies: %w", err)
			}

			assertElasticAPMEnabled(t, policies)
			return nil
		},
		beforeUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleILM, PreferIlm: true},
			},
		},
		afterUpgradeBeforeIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 1,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleILM, PreferIlm: true},
			},
		},
		afterUpgradeAfterIngest: checkDatastreamWant{
			Quantity:     8,
			PreferIlm:    true,
			DSManagedBy:  lifecycleILM,
			IndicesPerDs: 2,
			IndicesManagedBy: []checkDatastreamIndex{
				{ManagedBy: lifecycleILM, PreferIlm: true},
				{ManagedBy: lifecycleILM, PreferIlm: true},
			},
		},
	})
}
