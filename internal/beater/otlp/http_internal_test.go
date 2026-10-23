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
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater/request"
)

// testMaxBodySize stays above OTLP protobuf framing and a small gzip frame,
// so a rejection is the payload crossing the cap.
const testMaxBodySize = 256

func TestOTLPHTTPRequestBodyTooLarge(t *testing.T) {
	handlers := []struct {
		name   string
		handle func(HTTPHandlers, http.ResponseWriter, *http.Request)
	}{
		{name: "traces", handle: HTTPHandlers.HandleTraces},
		{name: "metrics", handle: HTTPHandlers.HandleMetrics},
		{name: "logs", handle: HTTPHandlers.HandleLogs},
	}
	for _, tc := range handlers {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveOTLP(t, newHTTPHandlers(t, testMaxBodySize), tc.handle, bytes.Repeat([]byte{0x0a}, testMaxBodySize+1), "")
			assertRequestBodyTooLarge(t, rec)
		})
	}
}

func TestOTLPHTTPRequestBodyAtLimit(t *testing.T) {
	// A body of exactly the limit passes the size check. These bytes are not
	// a valid OTLP payload, so decoding fails with 400.
	rec := serveOTLP(t, newHTTPHandlers(t, testMaxBodySize), HTTPHandlers.HandleTraces, bytes.Repeat([]byte{0x0a}, testMaxBodySize), "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOTLPHTTPRequestBodyTooLargeAfterDecompression(t *testing.T) {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName(string(bytes.Repeat([]byte("x"), 4<<10)))
	raw, err := ptraceotlp.NewExportRequestFromTraces(traces).MarshalProto()
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

	rec := serveOTLP(t, newHTTPHandlers(t, limit), HTTPHandlers.HandleTraces, compressed.Bytes(), "gzip")
	assertRequestBodyTooLarge(t, rec)
}

func TestOTLPHTTPUnfinishedChunkedRequestBody(t *testing.T) {
	// The client sends one byte past the limit and leaves the chunked body open.
	// The 413 must be readable without the terminating chunk. ResponseRecorder
	// cannot show this: it does not buffer the response or drain on Body.Close.
	var processed atomic.Bool
	handlers := NewHTTPHandlers(
		zap.NewNop(),
		modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
			processed.Store(true)
			return nil
		}),
		semaphore.NewWeighted(1),
		sdkmetric.NewMeterProvider(),
		noop.NewTracerProvider(),
	)
	handlers.maxBodySize = testMaxBodySize
	srv := httptest.NewServer(request.NewContextPool().HTTPHandler(func(c *request.Context) {
		handlers.HandleTraces(c.ResponseWriter, c.Request)
	}))
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	payload := bytes.Repeat([]byte{0x0a}, testMaxBodySize+1)
	_, err = fmt.Fprintf(conn, "POST /v1/traces HTTP/1.1\r\nHost: %s\r\nContent-Type: application/x-protobuf\r\nTransfer-Encoding: chunked\r\n\r\n%x\r\n", srv.Listener.Addr().String(), len(payload))
	require.NoError(t, err)
	_, err = conn.Write(payload)
	require.NoError(t, err)
	_, err = conn.Write([]byte("\r\n"))
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "413 was not flushed while the chunked body was still open")
	t.Cleanup(func() { resp.Body.Close() })
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, "413 Request Entity Too Large", resp.Status)
	// ReadResponse consumes Connection as a hop-by-hop header and sets Close.
	assert.True(t, resp.Close)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	want, err := proto.Marshal(status.New(codes.ResourceExhausted, errRequestBodyTooLarge.Error()).Proto())
	require.NoError(t, err)
	assert.Equal(t, want, body)
	assert.False(t, processed.Load())
}

func newHTTPHandlers(t *testing.T, maxBodySize int) HTTPHandlers {
	t.Helper()
	h := NewHTTPHandlers(
		zap.NewNop(),
		modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error {
			t.Fatal("request was processed")
			return nil
		}),
		semaphore.NewWeighted(1),
		sdkmetric.NewMeterProvider(),
		noop.NewTracerProvider(),
	)
	h.maxBodySize = maxBodySize
	return h
}

func serveOTLP(
	t *testing.T,
	h HTTPHandlers,
	handle func(HTTPHandlers, http.ResponseWriter, *http.Request),
	body []byte,
	contentEncoding string,
) *httptest.ResponseRecorder {
	t.Helper()
	// request.Context decompresses the body before the OTLP handler reads it.
	handler := request.NewContextPool().HTTPHandler(func(c *request.Context) {
		handle(h, c.ResponseWriter, c.Request)
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

func assertRequestBodyTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "close", rec.Header().Get("Connection"))
	want, err := proto.Marshal(status.New(codes.ResourceExhausted, errRequestBodyTooLarge.Error()).Proto())
	require.NoError(t, err)
	assert.Equal(t, want, rec.Body.Bytes())
}
