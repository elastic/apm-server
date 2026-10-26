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
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcgzip "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/internal/beater/beatertest"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/otlp"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

// testMaxBodySize stays above OTLP protobuf framing and a small gzip frame,
// so a rejection is the payload crossing the cap.
const testMaxBodySize = 256

func TestConsumeTracesGRPC(t *testing.T) {
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	conn := newGRPCServer(t, batchProcessor, mp)
	client := ptraceotlp.NewGRPCClient(conn)

	// Send a minimal trace to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operation_name")

	tracesRequest := ptraceotlp.NewExportRequestFromTraces(traces)
	_, err := client.Export(context.Background(), tracesRequest)
	assert.NoError(t, err)
	require.Len(t, batches, 1)

	reportError = errors.New("failed to publish events")
	_, err = client.Export(context.Background(), tracesRequest)
	assert.Error(t, err)
	errStatus := status.Convert(err)
	assert.Equal(t, "failed to publish events", errStatus.Message())
	require.Len(t, batches, 2)
	assert.Len(t, batches[0], 1)
	assert.Len(t, batches[1], 1)

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"apm-server.otlp.grpc.traces.request.count":         2,
		"apm-server.otlp.grpc.traces.response.valid.count":  1,
		"apm-server.otlp.grpc.traces.response.count":        2,
		"apm-server.otlp.grpc.traces.response.errors.count": 1,
	})
}

func TestConsumeMetricsGRPC(t *testing.T) {
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		return reportError
	}

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	conn := newGRPCServer(t, batchProcessor, mp)
	client := pmetricotlp.NewGRPCClient(conn)

	// Send a minimal metric to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	metrics := pmetric.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("metric_type")
	metric.SetEmptySummary()
	metric.Summary().DataPoints().AppendEmpty()

	metricsRequest := pmetricotlp.NewExportRequestFromMetrics(metrics)
	_, err := client.Export(context.Background(), metricsRequest)
	assert.NoError(t, err)

	reportError = errors.New("failed to publish events")
	_, err = client.Export(context.Background(), metricsRequest)
	assert.Error(t, err)

	errStatus := status.Convert(err)
	assert.Equal(t, "failed to publish events", errStatus.Message())

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"apm-server.otlp.grpc.metrics.request.count":         2,
		"apm-server.otlp.grpc.metrics.response.valid.count":  1,
		"apm-server.otlp.grpc.metrics.response.count":        2,
		"apm-server.otlp.grpc.metrics.response.errors.count": 1,
	})
}

func TestConsumeLogsGRPC(t *testing.T) {
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	conn := newGRPCServer(t, batchProcessor, mp)
	client := plogotlp.NewGRPCClient(conn)

	// Send a minimal log record to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	logsRequest := plogotlp.NewExportRequestFromLogs(logs)
	_, err := client.Export(context.Background(), logsRequest)
	assert.NoError(t, err)
	require.Len(t, batches, 1)

	reportError = errors.New("failed to publish events")
	_, err = client.Export(context.Background(), logsRequest)
	assert.Error(t, err)
	errStatus := status.Convert(err)
	assert.Equal(t, "failed to publish events", errStatus.Message())
	require.Len(t, batches, 2)
	assert.Len(t, batches[0], 1)
	assert.Len(t, batches[1], 1)

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"apm-server.otlp.grpc.logs.request.count":         2,
		"apm-server.otlp.grpc.logs.response.valid.count":  1,
		"apm-server.otlp.grpc.logs.response.count":        2,
		"apm-server.otlp.grpc.logs.response.errors.count": 1,
	})
}

func TestGRPCRequestTooLarge(t *testing.T) {
	var processed bool
	conn := newGRPCServerWithLimit(t, testMaxBodySize, modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
		processed = true
		return nil
	}), sdkmetric.NewMeterProvider())

	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(string(bytes.Repeat([]byte("x"), testMaxBodySize+1)))
	raw, err := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()
	require.NoError(t, err)
	require.Greater(t, len(span.Name()), testMaxBodySize)
	require.Greater(t, len(raw), testMaxBodySize)

	_, err = ptraceotlp.NewGRPCClient(conn).Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(traces))
	require.Error(t, err)
	st := status.Convert(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Empty(t, st.Details())
	assert.False(t, processed)
}

func TestGRPCRequestTooLargeAfterDecompression(t *testing.T) {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(string(bytes.Repeat([]byte("x"), 4<<10)))
	tracesRequest := ptraceotlp.NewExportRequestFromTraces(traces)
	raw, err := tracesRequest.MarshalProto()
	require.NoError(t, err)

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err = zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.Less(t, compressed.Len(), len(raw))

	// Stay above the gzip frame and below the decompressed message so a
	// failure is the post-decompression check, not the compressed-frame check.
	limit := compressed.Len() * 4
	require.Less(t, limit, len(raw))

	var processed bool
	conn := newGRPCServerWithLimit(t, limit, modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
		processed = true
		return nil
	}), sdkmetric.NewMeterProvider())

	_, err = ptraceotlp.NewGRPCClient(conn).Export(context.Background(), tracesRequest, grpc.UseCompressor(grpcgzip.Name))
	require.Error(t, err)
	st := status.Convert(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Empty(t, st.Details())
	assert.False(t, processed)
}

// TestBeaterGRPCRequestTooLarge installs testMaxBodySize on the server started
// by beater. The production server uses MaxRequestSize.
func TestBeaterGRPCRequestTooLarge(t *testing.T) {
	srv := beatertest.NewServer(t, beatertest.WithWrapServer(
		func(args beater.ServerParams, runServer beater.RunServerFunc) (beater.ServerParams, beater.RunServerFunc, error) {
			args.GRPCServer = grpc.NewServer(grpc.MaxRecvMsgSize(testMaxBodySize))
			return args, runServer, nil
		},
	))
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	conn, err := grpc.NewClient(u.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(string(bytes.Repeat([]byte("x"), testMaxBodySize+1)))
	raw, err := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()
	require.NoError(t, err)
	require.Greater(t, len(span.Name()), testMaxBodySize)
	require.Greater(t, len(raw), testMaxBodySize)

	_, err = ptraceotlp.NewGRPCClient(conn).Export(context.Background(), ptraceotlp.NewExportRequestFromTraces(traces))
	require.Error(t, err)
	st := status.Convert(err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Empty(t, st.Details())
}

func newGRPCServer(t *testing.T, batchProcessor modelpb.BatchProcessor, mp metric.MeterProvider) *grpc.ClientConn {
	return newGRPCServerWithLimit(t, otlp.MaxRequestSize, batchProcessor, mp)
}

func newGRPCServerWithLimit(t *testing.T, maxRecvMsgSize int, batchProcessor modelpb.BatchProcessor, mp metric.MeterProvider) *grpc.ClientConn {
	t.Helper()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	logger := logptest.NewTestingLogger(t, "otlp.grpc.test")
	srv := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxRecvMsgSize),
		grpc.UnaryInterceptor(interceptors.Metrics(logger, mp)),
	)
	semaphore := semaphore.NewWeighted(1)
	otlp.RegisterGRPCServices(srv, zap.NewNop(), batchProcessor, semaphore, mp, noop.NewTracerProvider())

	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}
