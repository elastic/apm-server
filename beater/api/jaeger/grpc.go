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

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	trjaeger "github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/otel"
)

const (
	collectorType = "jaeger"
)

var (
	gRPCRegistry   = monitoring.Default.NewRegistry("apm-server.jaeger.grpc", monitoring.PublishExpvar)
	monitoringKeys = []request.ResultID{request.IDRequestCount, request.IDResponseCount, request.IDResponseErrorsCount,
		request.IDResponseValidCount, request.IDEventReceivedCount, request.IDEventDroppedCount}
	monitoringMap = request.MonitoringMapForRegistry(gRPCRegistry, monitoringKeys)
)

// GRPCCollector implements Jaeger api_v2 protocol for receiving tracing data
type GRPCCollector struct {
	consumer *otel.Consumer
}

// NewGRPCCollector returns new instance of GRPCCollector
func NewGRPCCollector(consumer *otel.Consumer) GRPCCollector {
	return GRPCCollector{consumer}
}

// PostSpans implements the api_v2/collector.proto. It converts spans received via Jaeger Proto batch to open-telemetry
// TraceData and passes them on to the internal Consumer taking care of converting into Elastic APM format.
// The implementation of the protobuf contract is based on the open-telemetry implementation at
// https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/jaegerreceiver
func (c GRPCCollector) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	inc(request.IDRequestCount)

	resp, err := c.postSpans(ctx, r)

	inc(request.IDResponseCount)
	if err != nil {
		inc(request.IDResponseErrorsCount)
	} else {
		inc(request.IDResponseValidCount)
	}
	return resp, err
}

func (c GRPCCollector) postSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	spansCount := len(r.Batch.Spans)
	add(request.IDEventReceivedCount, int64(spansCount))
	traceData, err := trjaeger.ProtoBatchToOCProto(r.Batch)
	if err != nil {
		add(request.IDEventDroppedCount, int64(spansCount))
		return nil, err
	}
	traceData.SourceFormat = collectorType
	if err = c.consumer.ConsumeTraceData(ctx, traceData); err != nil {
		return nil, err
	}
	return &api_v2.PostSpansResponse{}, nil
}

func inc(id request.ResultID) {
	if counter, ok := monitoringMap[id]; ok {
		counter.Inc()
	}
}

func add(id request.ResultID, n int64) {
	if counter, ok := monitoringMap[id]; ok {
		counter.Add(n)
	}
}
