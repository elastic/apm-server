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

func TestUpgrade_8_15_4_to_8_16_0(t *testing.T) {
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
		// Check data streams and verify lazy rollover happened
		// v managed by DSL if created after 8.15.0
		// x managed by ILM if created before 8.15.0
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
