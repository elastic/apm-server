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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver/trace"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var (
	monitoringKeys = []request.ResultID{
		request.IDRequestCount, request.IDResponseCount, request.IDResponseErrorsCount, request.IDResponseValidCount,
	}

	gRPCConsumerRegistry      = monitoring.Default.NewRegistry("apm-server.otlp.grpc.consumer")
	gRPCConsumerMonitoringMap = request.MonitoringMapForRegistry(gRPCConsumerRegistry, monitoringKeys)
)

// RegisterGRPCServices registers OTLP consumer services with the given gRPC server.
func RegisterGRPCServices(grpcServer *grpc.Server, reporter publish.Reporter, logger *logp.Logger) error {
	consumer := &monitoredConsumer{
		consumer: &otel.Consumer{Reporter: reporter},
		logger:   logger,
	}
	// TODO(axw) add support for metrics to processer/otel.Consumer, and register a metrics receiver here.
	traceReceiver := trace.New("otlp", consumer)
	if err := otlpreceiver.RegisterTraceReceiver(context.Background(), traceReceiver, grpcServer, nil); err != nil {
		return errors.Wrap(err, "failed to register OTLP trace receiver")
	}
	return nil
}

type monitoredConsumer struct {
	consumer *otel.Consumer
	logger   *logp.Logger
}

// ConsumeTraces consumes OpenTelemtry trace data.
func (c *monitoredConsumer) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	gRPCConsumerMonitoringMap[request.IDRequestCount].Inc()
	defer gRPCConsumerMonitoringMap[request.IDResponseCount].Inc()
	if err := c.consumer.ConsumeTraces(ctx, traces); err != nil {
		gRPCConsumerMonitoringMap[request.IDResponseErrorsCount].Inc()
		c.logger.With(logp.Error(err)).Error("ConsumeTraces returned an error")
		return err
	}
	gRPCConsumerMonitoringMap[request.IDResponseValidCount].Inc()
	return nil
}
