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

package jaeger

import (
	"context"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
)

var (
	gRPCRegistry                    = monitoring.Default.NewRegistry("apm-server.jaeger.grpc", monitoring.PublishExpvar)
	gRPCMonitoringMap monitoringMap = request.MonitoringMapForRegistry(gRPCRegistry, monitoringKeys)
)

// grpcCollector implements Jaeger api_v2 protocol for receiving tracing data
type grpcCollector struct {
	auth     authFunc
	consumer consumer.TraceConsumer
}

// PostSpans implements the api_v2/collector.proto. It converts spans received via Jaeger Proto batch to open-telemetry
// TraceData and passes them on to the internal Consumer taking care of converting into Elastic APM format.
// The implementation of the protobuf contract is based on the open-telemetry implementation at
// https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/jaegerreceiver
func (c grpcCollector) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	gRPCMonitoringMap.inc(request.IDRequestCount)
	defer gRPCMonitoringMap.inc(request.IDResponseCount)

	if err := c.postSpans(ctx, r.Batch); err != nil {
		gRPCMonitoringMap.inc(request.IDResponseErrorsCount)
		return nil, err
	}
	gRPCMonitoringMap.inc(request.IDResponseValidCount)
	return &api_v2.PostSpansResponse{}, nil
}

func (c grpcCollector) postSpans(ctx context.Context, batch model.Batch) error {
	if err := c.auth(ctx, batch); err != nil {
		gRPCMonitoringMap.inc(request.IDResponseErrorsUnauthorized)
		return status.Error(codes.Unauthenticated, err.Error())
	}
	return consumeBatch(ctx, batch, c.consumer, gRPCMonitoringMap)
}
