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

package middleware

import (
	"net/http"
	"sync"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

var (
	mu sync.Mutex
)

type monitoringMapper struct {
	resultIDToCounter map[request.ResultID]*monitoring.Int
	registry          *monitoring.Registry
}

// MonitoringMiddleware returns a middleware that increases monitoring counters for collecting metrics
// about request processing. As input parameter it takes a map capable of mapping a request.ResultID to a counter.
func MonitoringMiddleware(registry *monitoring.Registry) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		mapper := monitoringMapper{resultIDToCounter: map[request.ResultID]*monitoring.Int{}, registry: registry}

		return func(c *request.Context) {
			mapper.inc(request.IDRequestCount)

			h(c)

			mapper.inc(request.IDResponseCount)
			if c.Result.StatusCode >= http.StatusBadRequest {
				mapper.inc(request.IDResponseErrorsCount)
			} else {
				mapper.inc(request.IDResponseValidCount)
			}

			mapper.inc(c.Result.ID)
		}, nil
	}
}

func (m *monitoringMapper) inc(id request.ResultID) {
	m.mapID(id).Inc()
}

func (m *monitoringMapper) mapID(id request.ResultID) *monitoring.Int {
	if ct, ok := m.resultIDToCounter[id]; ok {
		return ct
	}

	mu.Lock()
	defer mu.Unlock()
	if ct, ok := m.registry.Get(string(id)).(*monitoring.Int); ok {
		m.resultIDToCounter[id] = ct
		return ct
	}

	ct := monitoring.NewInt(m.registry, string(id))
	m.resultIDToCounter[id] = ct
	return ct
}
