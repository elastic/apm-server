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

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var (
	monitoringKeys = append(request.DefaultResultIDs,
		request.IDResponseErrorsRateLimit,
		request.IDResponseErrorsTimeout,
		request.IDResponseErrorsUnauthorized,
	)
	currentMonitoredConsumerMu sync.RWMutex
	currentMonitoredConsumer   *otel.Consumer
)

const (
	metricsFullMethod = "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export"
	tracesFullMethod  = "/opentelemetry.proto.collector.trace.v1.TraceService/Export"
	logsFullMethod    = "/opentelemetry.proto.collector.logs.v1.LogsService/Export"
)

func collectMetricsMonitoring(mode monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	currentMonitoredConsumerMu.RLock()
	c := currentMonitoredConsumer
	currentMonitoredConsumerMu.RUnlock()
	if c == nil {
		return
	}

	stats := c.Stats()
	monitoring.ReportInt(V, "unsupported_dropped", stats.UnsupportedMetricsDropped)
}

func setCurrentMonitoredConsumer(c *otel.Consumer) {
	currentMonitoredConsumerMu.Lock()
	defer currentMonitoredConsumerMu.Unlock()
	currentMonitoredConsumer = c
}
