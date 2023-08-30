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

package otlp

import (
	"sync"

	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var (
	monitoringKeys = append(request.DefaultResultIDs,
		request.IDResponseErrorsRateLimit,
		request.IDResponseErrorsTimeout,
		request.IDResponseErrorsUnauthorized,
	)
)

type monitoredConsumer struct {
	mu       sync.RWMutex
	consumer *otlp.Consumer
}

func (m *monitoredConsumer) set(c *otlp.Consumer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumer = c
}

func (m *monitoredConsumer) collect(mode monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	m.mu.RLock()
	c := m.consumer
	m.mu.RUnlock()
	if c == nil {
		return
	}

	stats := c.Stats()
	monitoring.ReportInt(V, "unsupported_dropped", stats.UnsupportedMetricsDropped)
}
