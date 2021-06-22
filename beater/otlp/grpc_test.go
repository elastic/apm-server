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

package otlp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

var (
	exportMetricsServiceRequestType  = proto.MessageType("opentelemetry.proto.collector.metrics.v1.ExportMetricsServiceRequest")
	exportMetricsServiceResponseType = proto.MessageType("opentelemetry.proto.collector.metrics.v1.ExportMetricsServiceResponse")
	exportTraceServiceRequestType    = proto.MessageType("opentelemetry.proto.collector.trace.v1.ExportTraceServiceRequest")
	exportTraceServiceResponseType   = proto.MessageType("opentelemetry.proto.collector.trace.v1.ExportTraceServiceResponse")
)

func TestConsumeTraces(t *testing.T) {
	var batches []*model.Batch
	var reportError error
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		batches = append(batches, batch)
		return reportError
	}

	// Send a minimal trace to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	cannedRequest := jsonExportTraceServiceRequest(`{
"resource_spans": [
  {
    "instrumentation_library_spans": [
      {
        "spans": [
	  {
	    "trace_id": "0123456789abcdef0123456789abcdef",
	    "span_id": "945254c567a5417e",
	    "name": "operation_name"
	  }
	]
      }
    ]
  }
]
}`)

	conn := newServer(t, batchProcessor)
	err := conn.Invoke(
		context.Background(), "/opentelemetry.proto.collector.trace.v1.TraceService/Export",
		cannedRequest, newExportTraceServiceResponse(),
	)
	assert.NoError(t, err)
	require.Len(t, batches, 1)

	reportError = errors.New("failed to publish events")
	err = conn.Invoke(
		context.Background(), "/opentelemetry.proto.collector.trace.v1.TraceService/Export",
		cannedRequest, newExportTraceServiceResponse(),
	)
	assert.Error(t, err)
	errStatus := status.Convert(err)
	assert.Equal(t, "failed to publish events", errStatus.Message())
	require.Len(t, batches, 2)

	for _, batch := range batches {
		assert.Equal(t, 1, batch.Len())
	}

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.grpc.traces").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		"request.count":                int64(2),
		"response.count":               int64(2),
		"response.errors.count":        int64(1),
		"response.valid.count":         int64(1),
		"response.errors.timeout":      int64(0),
		"response.errors.unauthorized": int64(0),
	}, actual)
}

func TestConsumeMetrics(t *testing.T) {
	var reportError error
	var batchProcessor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		return reportError
	}

	// Send a minimal metric to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	cannedRequest := jsonExportMetricsServiceRequest(`{
"resource_metrics": [
  {
    "instrumentation_library_metrics": [
      {
        "metrics": [
	  {
	    "name": "metric_name"
	  }
	]
      }
    ]
  }
]
}`)

	conn := newServer(t, batchProcessor)
	err := conn.Invoke(
		context.Background(), "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
		cannedRequest, newExportMetricsServiceResponse(),
	)
	assert.NoError(t, err)

	reportError = errors.New("failed to publish events")
	err = conn.Invoke(
		context.Background(), "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
		cannedRequest, newExportMetricsServiceResponse(),
	)
	assert.Error(t, err)
	errStatus := status.Convert(err)
	assert.Equal(t, "failed to publish events", errStatus.Message())

	actual := map[string]interface{}{}
	monitoring.GetRegistry("apm-server.otlp.grpc.metrics").Do(monitoring.Full, func(key string, value interface{}) {
		actual[key] = value
	})
	assert.Equal(t, map[string]interface{}{
		// In both of the requests we send above,
		// the metrics do not have a type and so
		// we treat them as unsupported metrics.
		"consumer.unsupported_dropped": int64(2),

		"request.count":                int64(2),
		"response.count":               int64(2),
		"response.errors.count":        int64(1),
		"response.valid.count":         int64(1),
		"response.errors.timeout":      int64(0),
		"response.errors.unauthorized": int64(0),
	}, actual)
}

func jsonExportTraceServiceRequest(j string) interface{} {
	request := reflect.New(exportTraceServiceRequestType.Elem()).Interface()
	decoder := json.NewDecoder(strings.NewReader(j))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		panic(err)
	}
	return request
}

func newExportTraceServiceResponse() interface{} {
	return reflect.New(exportTraceServiceResponseType.Elem()).Interface()
}

func jsonExportMetricsServiceRequest(j string) interface{} {
	request := reflect.New(exportMetricsServiceRequestType.Elem()).Interface()
	decoder := json.NewDecoder(strings.NewReader(j))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		panic(err)
	}
	return request
}

func newExportMetricsServiceResponse() interface{} {
	return reflect.New(exportMetricsServiceResponseType.Elem()).Interface()
}

func newServer(t *testing.T, batchProcessor model.BatchProcessor) *grpc.ClientConn {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	logger := logp.NewLogger("otlp.grpc.test")
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(interceptors.Metrics(logger, otlp.RegistryMonitoringMaps)),
	)
	err = otlp.RegisterGRPCServices(srv, batchProcessor)
	require.NoError(t, err)

	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}
