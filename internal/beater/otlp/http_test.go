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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/semaphore"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/api"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/middleware"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/otlp"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestConsumeTracesHTTP(t *testing.T) {
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr, reader := newHTTPServer(t, batchProcessor)

	// Send a minimal trace to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operation_name")

	tracesRequest := ptraceotlp.NewExportRequestFromTraces(traces)
	request, err := tracesRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/traces", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())
	require.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"http.server.request.count":        1,
		"http.server.response.count":       1,
		"http.server.response.valid.count": 1,
	})

}

func TestConsumeMetricsHTTP(t *testing.T) {
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		return reportError
	}

	addr, reader := newHTTPServer(t, batchProcessor)

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
	request, err := metricsRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/metrics", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"http.server.request.count":        1,
		"http.server.response.count":       1,
		"http.server.response.valid.count": 1,
	})
}

func TestConsumeLogsHTTP(t *testing.T) {
	var batches []modelpb.Batch
	var reportError error
	var batchProcessor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		batches = append(batches, *batch)
		return reportError
	}

	addr, reader := newHTTPServer(t, batchProcessor)

	// Send a minimal log record to verify that everything is connected properly.
	//
	// We intentionally do not check the published event contents; those are
	// tested in processor/otel.
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	logsRequest := plogotlp.NewExportRequestFromLogs(logs)
	request, err := logsRequest.MarshalProto()
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/v1/logs", addr), bytes.NewReader(request))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := http.Client{}
	rsp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, rsp.Body.Close())
	require.Len(t, batches, 1)

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"http.server.request.count":        1,
		"http.server.response.count":       1,
		"http.server.response.valid.count": 1,
	})
}

func newHTTPServer(t *testing.T, batchProcessor modelpb.BatchProcessor) (string, sdkmetric.Reader) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	cfg := &config.Config{}
	auth, _ := auth.NewAuthenticator(cfg.AgentAuth, noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	router, err := api.NewMux(
		cfg,
		batchProcessor,
		auth,
		agentcfg.NewEmptyFetcher(),
		ratelimitStore,
		nil,
		func() bool { return true },
		semaphore.NewWeighted(1),
		mp,
		noop.NewTracerProvider(),
		logptest.NewTestingLogger(t, ""),
		monitoring.NewRegistry(),
	)
	require.NoError(t, err)
	srv := http.Server{Handler: router}
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})
	go srv.Serve(lis)
	return lis.Addr().String(), reader
}

func TestOTLPHTTPRequestBodyTooLarge(t *testing.T) {
	const limit = 32
	handlers := []struct {
		name   string
		handle func(otlp.HTTPHandlers, *request.Context)
	}{
		{"traces", otlp.HTTPHandlers.HandleTraces},
		{"metrics", otlp.HTTPHandlers.HandleMetrics},
		{"logs", otlp.HTTPHandlers.HandleLogs},
	}
	for _, tc := range handlers {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveOTLP(t, newHTTPHandlers(t, limit), tc.handle, bytes.Repeat([]byte{0x0a}, limit+1), "")
			assertRequestBodyTooLarge(t, rec)
		})
	}
}

func TestOTLPHTTPRequestBodyAtLimit(t *testing.T) {
	const limit = 32
	// A body of exactly the limit passes the size check. These bytes are not
	// a valid OTLP payload, so decoding fails with 400.
	rec := serveOTLP(t, newHTTPHandlers(t, limit), otlp.HTTPHandlers.HandleTraces, bytes.Repeat([]byte{0x0a}, limit), "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOTLPHTTPRequestBodyLimitAfterDecompression(t *testing.T) {
	const limit = 32
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err := gz.Write(bytes.Repeat([]byte{0x0a}, limit*2))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.Less(t, compressed.Len(), limit, "compressed body must be under the limit so the check is on the decompressed size")

	rec := serveOTLP(t, newHTTPHandlers(t, limit), otlp.HTTPHandlers.HandleTraces, compressed.Bytes(), "gzip")
	assertRequestBodyTooLarge(t, rec)
}

func newHTTPHandlers(t *testing.T, maxBodySize int) otlp.HTTPHandlers {
	t.Helper()
	return otlp.NewHTTPHandlers(
		zap.NewNop(),
		modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
			t.Fatal("request was processed")
			return nil
		}),
		semaphore.NewWeighted(1),
		sdkmetric.NewMeterProvider(),
		noop.NewTracerProvider(),
		maxBodySize,
	)
}

func serveOTLP(
	t *testing.T,
	h otlp.HTTPHandlers,
	handle func(otlp.HTTPHandlers, *request.Context),
	body []byte,
	contentEncoding string,
) *httptest.ResponseRecorder {
	t.Helper()
	// request.Context decompresses the body before the OTLP handler reads it.
	handler := request.NewContextPool().HTTPHandler(func(c *request.Context) {
		handle(h, c)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestOTLPHTTPErrorMonitoring(t *testing.T) {
	const limit = 32
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	validBody, err := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		body         []byte
		processErr   error
		expectStatus int
		expectResult string
	}{
		"invalid_body": {
			body:         []byte("not protobuf"),
			expectStatus: http.StatusBadRequest,
			expectResult: "http.server.response.errors.decode",
		},
		"too_large": {
			body:         bytes.Repeat([]byte{0x0a}, limit+1),
			expectStatus: http.StatusRequestEntityTooLarge,
			expectResult: "http.server.response.errors.toolarge",
		},
		"processor_error": {
			body:         validBody,
			processErr:   errors.New("processor failed"),
			expectStatus: http.StatusInternalServerError,
			expectResult: "http.server.response.errors.internal",
		},
		"unauthorized": {
			body:         validBody,
			processErr:   auth.ErrUnauthorized,
			expectStatus: http.StatusForbidden,
			expectResult: "http.server.response.errors.forbidden",
		},
	} {
		t.Run(name, func(t *testing.T) {
			handlers := otlp.NewHTTPHandlers(
				zap.NewNop(),
				modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
					return tc.processErr
				}),
				semaphore.NewWeighted(1),
				sdkmetric.NewMeterProvider(),
				noop.NewTracerProvider(),
				limit,
			)
			logger, observedLogs := logptest.NewTestingLoggerWithObserver(t, "")
			reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
				func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
					return metricdata.DeltaTemporality
				},
			))
			mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			handler, err := middleware.Wrap(
				func(c *request.Context) { handlers.HandleTraces(c) },
				middleware.LogMiddleware(logger),
				middleware.MonitoringMiddleware("apm-server.otlp.http.traces.", mp),
			)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-protobuf")
			rec := httptest.NewRecorder()
			request.NewContextPool().HTTPHandler(handler).ServeHTTP(rec, req)
			assert.Equal(t, tc.expectStatus, rec.Code)

			monitoringtest.ExpectContainAndNotContainOtelMetrics(t, reader, map[string]any{
				"http.server.request.count":         1,
				"http.server.response.count":        1,
				"http.server.response.errors.count": 1,
				tc.expectResult:                     1,
			}, []string{"http.server.response.valid.count"})

			requestLogs := observedLogs.FilterLoggerName("request").All()
			require.Len(t, requestLogs, 1)
			assert.Equal(t, zapcore.ErrorLevel, requestLogs[0].Level)
			assert.Equal(t, int64(tc.expectStatus), requestLogs[0].ContextMap()["http.response.status_code"])
		})
	}
}

func assertRequestBodyTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "close", rec.Header().Get("Connection"))
	var st statuspb.Status
	require.NoError(t, proto.Unmarshal(rec.Body.Bytes(), &st))
	assert.Equal(t, int32(codes.ResourceExhausted), st.GetCode())
	assert.Equal(t, "request body too large", st.GetMessage())
	assert.Empty(t, st.GetDetails())
}
