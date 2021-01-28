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

	gatewayruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"

	collectorlog "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/collector/trace/v1"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/logs"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/metrics"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/trace"
)

// RegisterTraceReceiver registers the trace receiver with a gRPC server and/or grpc-gateway mux, if non-nil.
func RegisterTraceReceiver(ctx context.Context, receiver *trace.Receiver, serverGRPC *grpc.Server, gatewayMux *gatewayruntime.ServeMux) error {
	if serverGRPC != nil {
		collectortrace.RegisterTraceServiceServer(serverGRPC, receiver)
	}
	if gatewayMux != nil {
		err := collectortrace.RegisterTraceServiceHandlerServer(ctx, gatewayMux, receiver)
		if err != nil {
			return err
		}
		// Also register an alias handler. This fixes bug https://github.com/open-telemetry/opentelemetry-collector/issues/1968
		return collectortrace.RegisterTraceServiceHandlerServerAlias(ctx, gatewayMux, receiver)
	}
	return nil
}

// RegisterMetricsReceiver registers the metrics receiver with a gRPC server and/or grpc-gateway mux, if non-nil.
func RegisterMetricsReceiver(ctx context.Context, receiver *metrics.Receiver, serverGRPC *grpc.Server, gatewayMux *gatewayruntime.ServeMux) error {
	if serverGRPC != nil {
		collectormetrics.RegisterMetricsServiceServer(serverGRPC, receiver)
	}
	if gatewayMux != nil {
		return collectormetrics.RegisterMetricsServiceHandlerServer(ctx, gatewayMux, receiver)
	}
	return nil
}

// RegisterLogsReceiver registers the logs receiver with a gRPC server and/or grpc-gateway mux, if non-nil.
func RegisterLogsReceiver(ctx context.Context, receiver *logs.Receiver, serverGRPC *grpc.Server, gatewayMux *gatewayruntime.ServeMux) error {
	if serverGRPC != nil {
		collectorlog.RegisterLogsServiceServer(serverGRPC, receiver)
	}
	if gatewayMux != nil {
		return collectorlog.RegisterLogsServiceHandlerServer(ctx, gatewayMux, receiver)
	}
	return nil
}
