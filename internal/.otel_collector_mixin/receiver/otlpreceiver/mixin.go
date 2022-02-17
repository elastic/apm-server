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

	"go.opentelemetry.io/otel/metric"
	apitrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlpgrpc"
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

// RegisterTraceReceiver registers the trace receiver with a gRPC server.
func RegisterTraceReceiver(ctx context.Context, consumer consumer.Traces, serverGRPC *grpc.Server) error {

	receiver := trace.New(config.NewComponentID("otlp"), consumer, settings)
	otlpgrpc.RegisterTracesServer(serverGRPC, receiver)
	return nil
}

// RegisterMetricsReceiver registers the metrics receiver with a gRPC server.
func RegisterMetricsReceiver(ctx context.Context, consumer consumer.Metrics, serverGRPC *grpc.Server) error {
	receiver := metrics.New(config.NewComponentID("otlp"), consumer, settings)
	otlpgrpc.RegisterMetricsServer(serverGRPC, receiver)
	return nil
}

// RegisterLogsReceiver registers the logs receiver with a gRPC server.
func RegisterLogsReceiver(ctx context.Context, consumer consumer.Logs, serverGRPC *grpc.Server) error {
	receiver := logs.New(config.NewComponentID("otlp"), consumer, settings)
	otlpgrpc.RegisterLogsServer(serverGRPC, receiver)
	return nil
}
