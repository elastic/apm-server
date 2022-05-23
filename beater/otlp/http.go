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
	"context"

	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var (
	httpMetricsRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.http.metrics")
	HTTPMetricsMonitoringMap = request.MonitoringMapForRegistry(httpMetricsRegistry, monitoringKeys)
	httpTracesRegistry       = monitoring.Default.NewRegistry("apm-server.otlp.http.traces")
	HTTPTracesMonitoringMap  = request.MonitoringMapForRegistry(httpTracesRegistry, monitoringKeys)
	httpLogsRegistry         = monitoring.Default.NewRegistry("apm-server.otlp.http.logs")
	HTTPLogsMonitoringMap    = request.MonitoringMapForRegistry(httpLogsRegistry, monitoringKeys)
)

func init() {
	monitoring.NewFunc(httpMetricsRegistry, "consumer", collectMetricsMonitoring, monitoring.Report)
}

func NewHTTPHandlers(processor model.BatchProcessor) (*otlpreceiver.HTTPHandlers, error) {
	consumer := &otel.Consumer{Processor: processor}
	setCurrentMonitoredConsumer(consumer)

	tracesHandler, err := otlpreceiver.TracesHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP trace receiver")
	}
	metricsHandler, err := otlpreceiver.MetricsHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP metrics receiver")
	}
	logsHandler, err := otlpreceiver.LogsHTTPHandler(context.Background(), consumer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create OTLP logs receiver")
	}
	return &otlpreceiver.HTTPHandlers{
		TraceHandler:   tracesHandler,
		MetricsHandler: metricsHandler,
		LogsHandler:    logsHandler,
	}, nil
}
