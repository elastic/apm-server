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

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

// CompareMonitoringInt matches expected with real monitoring counters and
// returns false and an a string showind diffs if not matching
func CompareMonitoringInt(
	handler func(c *request.Context),
	c *request.Context,
	expected map[request.ResultID]int,
	registry *monitoring.Registry,
	mapFn func(id request.ResultID) *monitoring.Int,
) (bool, string) {

	clearRegistry(registry, mapFn)
	handler(c)

	var result string
	for _, id := range AllRequestResultIDs() {
		monitoringIntVal := int64(0)
		monitoringInt := mapFn(id)
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
	return []request.ResultID{
		request.IDUnset,
		request.IDRequestCount,
		request.IDResponseCount,
		request.IDResponseErrorsCount,
		request.IDResponseValidCount,
		request.IDResponseValidNotModified,
		request.IDResponseValidOK,
		request.IDResponseValidAccepted,
		request.IDResponseErrorsForbidden,
		request.IDResponseErrorsUnauthorized,
		request.IDResponseErrorsNotFound,
		request.IDResponseErrorsInvalidQuery,
		request.IDResponseErrorsRequestTooLarge,
		request.IDResponseErrorsDecode,
		request.IDResponseErrorsValidate,
		request.IDResponseErrorsRateLimit,
		request.IDResponseErrorsMethodNotAllowed,
		request.IDResponseErrorsFullQueue,
		request.IDResponseErrorsShuttingDown,
		request.IDResponseErrorsServiceUnavailable,
		request.IDResponseErrorsInternal}
}

// clearRegistry sets all counters to 0 and removes all registered counters from the registry
// Only use this in test environments
func clearRegistry(r *monitoring.Registry, fn func(id request.ResultID) *monitoring.Int) {
	for _, id := range AllRequestResultIDs() {
		i := fn(id)
		if i != nil {
			i.Set(0)
		}
	}
	r.Clear()
}
