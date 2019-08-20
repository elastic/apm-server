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

package intake

import (
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

var (
	// MonitoringRegistry monitoring registry for Intake API events
	MonitoringRegistry = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	resultIDToCounter  = fillResultIDToCounter()
)

// ResultIDToMonitoringInt takes a request.ResultID and maps it to a monitoring counter. If the ID is UnsetID,
// nil is returned.
func ResultIDToMonitoringInt(name request.ResultID) *monitoring.Int {
	if i, ok := resultIDToCounter[name]; ok {
		return i
	}
	return nil
}

func fillResultIDToCounter() map[request.ResultID]*monitoring.Int {
	counter := func(s request.ResultID) *monitoring.Int {
		return monitoring.NewInt(MonitoringRegistry, string(s))
	}

	m := map[request.ResultID]*monitoring.Int{}
	for id := range request.MapResultIDToStatus {
		if id == request.IDUnset {
			//TODO: remove this to also count unset IDs as indicator that some ID has not been set
			continue
		}
		m[id] = counter(id)
	}
	return m
}
