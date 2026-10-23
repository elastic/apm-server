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
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestOTLPHTTPRequestBodyTooLarge(t *testing.T) {
	const limit = 32
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
			rec := serveOTLP(t, newHTTPHandlers(t, limit), tc.handle, bytes.Repeat([]byte{0x0a}, limit+1), "")
			assertRequestBodyTooLarge(t, rec)
		})
	}
}

func TestOTLPHTTPRequestBodyAtLimit(t *testing.T) {
	const limit = 32
	// A body of exactly the limit passes the size check. These bytes are not
	// a valid OTLP payload, so decoding fails with 400.
	rec := serveOTLP(t, newHTTPHandlers(t, limit), HTTPHandlers.HandleTraces, bytes.Repeat([]byte{0x0a}, limit), "")
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

	rec := serveOTLP(t, newHTTPHandlers(t, limit), HTTPHandlers.HandleTraces, compressed.Bytes(), "gzip")
	assertRequestBodyTooLarge(t, rec)
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
