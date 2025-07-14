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
	"sync"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/input/otlp"
	"github.com/elastic/apm-data/model/modelpb"
)

var (
	grpcMetricRegistrationMu          sync.Mutex
	unsupportedGRPCMetricRegistration metric.Registration
)

// RegisterGRPCServices registers OTLP consumer services with the given gRPC server.
func RegisterGRPCServices(
	grpcServer *grpc.Server,
	logger *zap.Logger,
	processor modelpb.BatchProcessor,
	semaphore input.Semaphore,
	mp metric.MeterProvider,
	tp trace.TracerProvider,
) {
	// TODO(axw) stop assuming we have only one OTLP gRPC service running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	consumer := otlp.NewConsumer(otlp.ConsumerConfig{
		Processor:        processor,
		Logger:           logger,
		Semaphore:        semaphore,
		RemapOTelMetrics: true,
		TraceProvider:    tp,
	})

	meter := mp.Meter("github.com/elastic/apm-server/internal/beater/otlp")
	grpcMetricsConsumerUnsupportedDropped, _ := meter.Int64ObservableCounter(
		"apm-server.otlp.grpc.metrics.consumer.unsupported_dropped",
	)

	grpcMetricRegistrationMu.Lock()
	defer grpcMetricRegistrationMu.Unlock()

	// TODO we should add an otel counter metric directly in the
	// apm-data consumer, then we could get rid of the callback.
	if unsupportedGRPCMetricRegistration != nil {
		_ = unsupportedGRPCMetricRegistration.Unregister()
	}
	unsupportedGRPCMetricRegistration, _ = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		stats := consumer.Stats()
		if stats.UnsupportedMetricsDropped > 0 {
			o.ObserveInt64(grpcMetricsConsumerUnsupportedDropped, stats.UnsupportedMetricsDropped)
		}
		return nil
	}, grpcMetricsConsumerUnsupportedDropped)

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
	resp := ptraceotlp.NewExportResponse()
	result, err := s.consumer.ConsumeTracesWithResult(ctx, td)
	if err == nil && result.RejectedSpans > 0 {
		resp.PartialSuccess().SetRejectedSpans(result.RejectedSpans)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	return resp, err
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
	resp := pmetricotlp.NewExportResponse()
	result, err := s.consumer.ConsumeMetricsWithResult(ctx, md)
	if err == nil && result.RejectedDataPoints > 0 {
		resp.PartialSuccess().SetRejectedDataPoints(result.RejectedDataPoints)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	return resp, err
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
	resp := plogotlp.NewExportResponse()
	result, err := s.consumer.ConsumeLogsWithResult(ctx, ld)
	if err == nil && result.RejectedLogRecords > 0 {
		resp.PartialSuccess().SetRejectedLogRecords(result.RejectedLogRecords)
		resp.PartialSuccess().SetErrorMessage(result.ErrorMessage)
	}
	return resp, err
}
