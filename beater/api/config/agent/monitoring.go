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

package agent

import (
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/api/intake"
	"github.com/elastic/apm-server/beater/request"
)

var (

	//TODO: change logic for acm specific monitoring counters
	//will be done in a follow up PR to avoid changing logic here
	//serverMetrics = monitoring.Default.NewRegistry("apm-server.server.acm", monitoring.PublishExpvar)
	//counter       = func(s request.ResultID) *monitoring.Int {
	//	return monitoring.NewInt(serverMetrics, string(s))
	//}

	// reflects current behavior
	mapping = map[request.ResultID]*monitoring.Int{
		request.IDRequestCount: intake.ResultIDToMonitoringInt(request.IDRequestCount),
	}
)

// ResultIDToMonitoringInt takes a request.ResultID and maps it to a monitoring counter. If no mapping is found,
// nil is returned.
func ResultIDToMonitoringInt(id request.ResultID) *monitoring.Int {
	if i, ok := mapping[id]; ok {
		return i
	}
	return nil
}
