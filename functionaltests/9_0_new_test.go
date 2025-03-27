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

func TestUpgrade_8_18_to_9_0_Snapshot_New(t *testing.T) {
	t.Parallel()

	// All data streams in this upgrade should be ILM, with no rollover.
	checkILM := checkDataStreamWant{
		Quantity:         8,
		PreferIlm:        true,
		DSManagedBy:      managedByILM,
		IndicesPerDs:     1,
		IndicesManagedBy: []string{managedByILM},
	}

	r := testStepsRunner{
		startVersion: getLatestSnapshot(t, "8.18"),
		steps: []testStep{
			ingestionStep{
				checkDataStream: checkILM,
			},
			upgradeStep{
				newVersion:      getLatestSnapshot(t, "9.0"),
				checkDataStream: checkILM,
			},
			ingestionStep{
				checkDataStream: checkILM,
			},
			checkErrorLogsStep{
				apmErrorLogsIgnored: []types.Query{
					tlsHandshakeError,
					esReturnedUnknown503,
					refreshCache503,
					// TODO: remove once fixed
					populateSourcemapFetcher403,
				},
			},
		},
	}

	r.Run(t)
}
