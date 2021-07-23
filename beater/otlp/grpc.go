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

	"github.com/pkg/errors"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
)

var (
	monitoringKeys = append(request.DefaultResultIDs,
		request.IDResponseErrorsRateLimit,
		request.IDResponseErrorsTimeout,
		request.IDResponseErrorsUnauthorized,
	)
	gRPCMetricsRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.grpc.metrics")
	gRPCMetricsMonitoringMap = request.MonitoringMapForRegistry(gRPCMetricsRegistry, monitoringKeys)
	gRPCTracesRegistry       = monitoring.Default.NewRegistry("apm-server.otlp.grpc.traces")
	gRPCTracesMonitoringMap  = request.MonitoringMapForRegistry(gRPCTracesRegistry, monitoringKeys)

	// RegistryMonitoringMaps provides mappings from the fully qualified gRPC
	// method name to its respective monitoring map.
	RegistryMonitoringMaps = map[string]map[request.ResultID]*monitoring.Int{
		metricsFullMethod: gRPCMetricsMonitoringMap,
		tracesFullMethod:  gRPCTracesMonitoringMap,
	}
)

const (
	metricsFullMethod = "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export"
	tracesFullMethod  = "/opentelemetry.proto.collector.trace.v1.TraceService/Export"
)

func init() {
	monitoring.NewFunc(gRPCMetricsRegistry, "consumer", collectMetricsMonitoring, monitoring.Report)
}

// MethodAuthenticators returns a map of all supported OTLP/gRPC methods to authenticators.
func MethodAuthenticators(authenticator *auth.Authenticator) map[string]interceptors.MethodAuthenticator {
	metadataMethodAuthenticator := interceptors.MetadataMethodAuthenticator(authenticator)
	return map[string]interceptors.MethodAuthenticator{
		metricsFullMethod: metadataMethodAuthenticator,
		tracesFullMethod:  metadataMethodAuthenticator,
	}
}

// RegisterGRPCServices registers OTLP consumer services with the given gRPC server.
func RegisterGRPCServices(grpcServer *grpc.Server, processor model.BatchProcessor) error {
	consumer := &otel.Consumer{Processor: processor}

	// TODO(axw) stop assuming we have only one OTLP gRPC service running
	// at any time, and instead aggregate metrics from consumers that are
	// dynamically registered and unregistered.
	setCurrentMonitoredConsumer(consumer)

	if err := otlpreceiver.RegisterTraceReceiver(context.Background(), consumer, grpcServer); err != nil {
		return errors.Wrap(err, "failed to register OTLP trace receiver")
	}
	if err := otlpreceiver.RegisterMetricsReceiver(context.Background(), consumer, grpcServer); err != nil {
		return errors.Wrap(err, "failed to register OTLP metrics receiver")
	}
	return nil
}

var (
	currentMonitoredConsumerMu sync.RWMutex
	currentMonitoredConsumer   *otel.Consumer
)

func setCurrentMonitoredConsumer(c *otel.Consumer) {
	currentMonitoredConsumerMu.Lock()
	defer currentMonitoredConsumerMu.Unlock()
	currentMonitoredConsumer = c
}

func collectMetricsMonitoring(mode monitoring.Mode, V monitoring.Visitor) {
	currentMonitoredConsumerMu.RLock()
	c := currentMonitoredConsumer
	currentMonitoredConsumerMu.RUnlock()
	if c == nil {
		return
	}

	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	stats := c.Stats()
	monitoring.ReportInt(V, "unsupported_dropped", stats.UnsupportedMetricsDropped)
}
