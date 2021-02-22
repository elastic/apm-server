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

package beatertest

import (
	"fmt"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

// CompareMonitoringInt matches expected with real monitoring counters and
// returns false and an a string showind diffs if not matching.
//
// The caller is expected to call ClearRegistry before invoking some code
// path that should update monitoring counters.
func CompareMonitoringInt(
	expected map[request.ResultID]int,
	m map[request.ResultID]*monitoring.Int,
) (bool, string) {
	var result string
	for _, id := range AllRequestResultIDs() {
		monitoringIntVal := int64(0)
		monitoringInt := m[id]
		if monitoringInt != nil {
			monitoringIntVal = monitoringInt.Get()
		}
		expectedVal := int64(0)
		if val, included := expected[id]; included {
			expectedVal = int64(val)
		}
		if expectedVal != monitoringIntVal {
			result += fmt.Sprintf("[%s] Expected: %d, Received: %d", id, expectedVal, monitoringIntVal)
		}
	}
	return len(result) == 0, result
}

// AllRequestResultIDs returns all registered request.ResultIDs (needs to be manually maintained)
func AllRequestResultIDs() []request.ResultID {
	var ids []request.ResultID
	for k := range request.MapResultIDToStatus {
		ids = append(ids, k)
	}
	return ids
}

// ClearRegistry sets all counters to 0 and removes all registered counters from the registry
// Only use this in test environments
func ClearRegistry(m map[request.ResultID]*monitoring.Int) {
	for _, i := range m {
		if i != nil {
			i.Set(0)
		}
	}
}
