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
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"google.golang.org/grpc"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/processor/otel"
)

var (
	gRPCMetricsRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.grpc.metrics")
	gRPCMetricsMonitoringMap = request.MonitoringMapForRegistry(gRPCMetricsRegistry, monitoringKeys)
	gRPCTracesRegistry       = monitoring.Default.NewRegistry("apm-server.otlp.grpc.traces")
	gRPCTracesMonitoringMap  = request.MonitoringMapForRegistry(gRPCTracesRegistry, monitoringKeys)
	gRPCLogsRegistry         = monitoring.Default.NewRegistry("apm-server.otlp.grpc.logs")
	gRPCLogsMonitoringMap    = request.MonitoringMapForRegistry(gRPCLogsRegistry, monitoringKeys)

	gRPCMonitoredConsumer monitoredConsumer
)

func init() {
	monitoring.NewFunc(gRPCMetricsRegistry, "consumer", gRPCMonitoredConsumer.collect, monitoring.Report)

	interceptors.RegisterMethodUnaryRequestMetrics(
		"/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
		gRPCMetricsMonitoringMap,
	)
	interceptors.RegisterMethodUnaryRequestMetrics(
		"/opentelemetry.proto.collector.trace.v1.TraceService/Export",
		gRPCTracesMonitoringMap,
	)
	interceptors.RegisterMethodUnaryRequestMetrics(
		"/opentelemetry.proto.collector.logs.v1.LogsService/Export",
		gRPCLogsMonitoringMap,
	)
}

// RegisterGRPCServices registers OTLP consumer services with the given gRPC server.
func RegisterGRPCServices(grpcServer *grpc.Server, processor model.BatchProcessor) {
	// TODO(axw) stop assuming we have only one OTLP gRPC service running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := &otel.Consumer{Processor: processor}
	gRPCMonitoredConsumer.set(consumer)

	tracesService := otlpreceiver.TracesService(consumer)
	ptraceotlp.RegisterGRPCServer(grpcServer, tracesService)

	metricsService := otlpreceiver.MetricsService(consumer)
	pmetricotlp.RegisterGRPCServer(grpcServer, metricsService)

	logsService := otlpreceiver.LogsService(consumer)
	plogotlp.RegisterGRPCServer(grpcServer, logsService)
}
