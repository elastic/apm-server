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

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/request"
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
func RegisterGRPCServices(
	grpcServer *grpc.Server,
	logger *zap.Logger,
	processor modelpb.BatchProcessor,
	semaphore input.Semaphore,
) {
	// TODO(axw) stop assuming we have only one OTLP gRPC service running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor: processor,
		Logger:    logger,
		Semaphore: semaphore,
	})
	gRPCMonitoredConsumer.set(consumer)

	ptraceotlp.RegisterGRPCServer(grpcServer, &tracesService{consumer: consumer})
	pmetricotlp.RegisterGRPCServer(grpcServer, &metricsService{consumer: consumer})
	plogotlp.RegisterGRPCServer(grpcServer, &logsService{consumer: consumer})
}

type tracesService struct {
	ptraceotlp.UnimplementedGRPCServer
	consumer *otlp.Consumer
}

func (s *tracesService) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	td := req.Traces()
	if td.SpanCount() == 0 {
		return ptraceotlp.NewExportResponse(), nil
	}
	err := s.consumer.ConsumeTraces(ctx, td)
	return ptraceotlp.NewExportResponse(), err
}

type metricsService struct {
	pmetricotlp.UnimplementedGRPCServer
	consumer *otlp.Consumer
}

func (s *metricsService) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	if md.DataPointCount() == 0 {
		return pmetricotlp.NewExportResponse(), nil
	}
	err := s.consumer.ConsumeMetrics(ctx, md)
	return pmetricotlp.NewExportResponse(), err
}

type logsService struct {
	plogotlp.UnimplementedGRPCServer
	consumer *otlp.Consumer
}

func (s *logsService) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	ld := req.Logs()
	if ld.LogRecordCount() == 0 {
		return plogotlp.NewExportResponse(), nil
	}
	err := s.consumer.ConsumeLogs(ctx, ld)
	return plogotlp.NewExportResponse(), err
}
