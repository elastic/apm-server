// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlpreceiver

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/metric"
	apitrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/internal/logs"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/internal/metrics"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/internal/trace"
)

// TODO(axw) pass this into the Register*Receiver functions, so we can pass in a logger,
// the apm-server version, etc.
var settings = component.ReceiverCreateSettings{
	TelemetrySettings: component.TelemetrySettings{
		Logger:         zap.NewNop(),
		TracerProvider: apitrace.NewNoopTracerProvider(),
		MeterProvider:  metric.NewNoopMeterProvider(),
		MetricsLevel:   configtelemetry.LevelNone,
	},
	BuildInfo: component.BuildInfo{
		Command:     "apm-server",
		Description: "Elastic APM Server",
		Version:     "latest",
	},
}

type HTTPHandlers struct {
	TraceHandler   http.HandlerFunc
	MetricsHandler http.HandlerFunc
	LogsHandler    http.HandlerFunc
}

// GRPC Receivers

// TracesService returns a ptraceotlp.Server for registering with a gRPC server,
// which will send trace data to the consumer.
func TracesService(consumer consumer.Traces) ptraceotlp.GRPCServer {
	return trace.New(config.NewComponentID("otlp"), consumer, settings)
}

// MetricsService returns a pmetricotlp.Server for registering with a gRPC server,
// which will send metrics data to the consumer.
func MetricsService(consumer consumer.Metrics) pmetricotlp.GRPCServer {
	return metrics.New(config.NewComponentID("otlp"), consumer, settings)
}

// LogsService returns a plogotlp.Server for registering with a gRPC server,
// which will send logs data to the consumer.
func LogsService(consumer consumer.Logs) plogotlp.GRPCServer {
	return logs.New(config.NewComponentID("otlp"), consumer, settings)
}

// HTTP Receivers

func TracesHTTPHandler(ctx context.Context, consumer consumer.Traces) (http.HandlerFunc, error) {
	receiver := trace.New(config.NewComponentID("otlp"), consumer, settings)
	return func(w http.ResponseWriter, r *http.Request) {
		handleTraces(w, r, receiver, pbEncoder)
	}, nil
}

func MetricsHTTPHandler(ctx context.Context, consumer consumer.Metrics) (http.HandlerFunc, error) {
	receiver := metrics.New(config.NewComponentID("otlp"), consumer, settings)
	return func(w http.ResponseWriter, r *http.Request) {
		handleMetrics(w, r, receiver, pbEncoder)
	}, nil
}

func LogsHTTPHandler(ctx context.Context, consumer consumer.Logs) (http.HandlerFunc, error) {
	receiver := logs.New(config.NewComponentID("otlp"), consumer, settings)
	return func(w http.ResponseWriter, r *http.Request) {
		handleLogs(w, r, receiver, pbEncoder)
	}, nil
}
