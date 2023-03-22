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

package eventhandler

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type mockServer struct {
	got      *bytes.Buffer
	close    func()
	received uint
}

func (t *mockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if t.got == nil {
		http.Error(w, "buffer not specified", 500)
		return
	}
	var reader io.Reader
	switch r.Header.Get("Content-Encoding") {
	case "deflate":
		zreader, err := zlib.NewReader(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("zlib.NewReader(): %v", err), 400)
			return
		}
		defer zreader.Close()
		reader = zreader
	default:
		reader = r.Body
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		isMeta := bytes.HasPrefix(line, metaHeader) ||
			bytes.HasPrefix(line, rumMetaHeader)
		if !isMeta {
			t.received++
		}
		t.got.Write(line)
		t.got.Write([]byte("\n"))
	}
	w.WriteHeader(http.StatusAccepted)
}

func newHandler(tb testing.TB, dir, expr string, l *rate.Limiter) (*Handler, *mockServer) {
	tb.Helper()
	ms := &mockServer{got: &bytes.Buffer{}}
	srv := httptest.NewServer(ms)
	ms.close = srv.Close
	transp := NewTransport(srv.Client(), srv.URL, "", "")
	h, err := New(Config{
		Path:      expr,
		Transport: transp,
		Storage:   os.DirFS(dir),
		Limiter:   l,
	})
	require.NoError(tb, err)
	return h, ms
}

func TestHandlerNew(t *testing.T) {
	storage := os.DirFS("testdata")
	t.Run("success-matches-files", func(t *testing.T) {
		h, err := New(Config{
			Path:      `python*.ndjson`,
			Transport: &Transport{},
			Storage:   storage,
		})
		require.NoError(t, err)
		assert.Greater(t, len(h.batches), 0)
	})
	t.Run("failure-matches-no-files", func(t *testing.T) {
		h, err := New(Config{
			Path:      `go*.ndjson`,
			Transport: &Transport{},
			Storage:   storage,
		})
		require.EqualError(t, err, "eventhandler: glob matched no files, please specify a valid glob pattern")
		assert.Nil(t, h)
	})
	t.Run("failure-invalid-glob", func(t *testing.T) {
		h, err := New(Config{
			Path:      "",
			Transport: &Transport{},
			Storage:   storage,
		})
		require.EqualError(t, err, "eventhandler: glob matched no files, please specify a valid glob pattern")
		assert.Nil(t, h)
	})
	t.Run("failure-rum-data", func(t *testing.T) {
		storage := os.DirFS(filepath.Join("..", "..", "..", "testdata", "intake-v3"))
		h, err := New(Config{
			Path:      `*.ndjson`,
			Transport: &Transport{},
			Storage:   storage,
		})
		require.EqualError(t, err, "rum data support not implemented")
		assert.Nil(t, h)
	})
}

func TestHandlerSendBatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(rate.Inf, 0))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		b, err := os.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), srv.got.String()) // Ensure the contents match.
		assert.Equal(t, uint(32), srv.received)
		assert.Equal(t, uint(32), n)             // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
	})
	t.Run("cancel-before-sendbatches", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(rate.Inf, 0))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, srv.received, uint(0))
		assert.Equal(t, n, uint(0))
	})
	t.Run("returns-error", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", `python*.ndjson`, rate.NewLimiter(rate.Inf, 0))
		// Close the server prematurely to force an error.
		srv.close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, n, uint(0))
	})
	t.Run("success-with-rate-limit", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(9, 16))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		// 16 + 9 (1st sec) + 9 (2nd sec) > 32 (total send)
		assert.NoError(t, err)

		b, err := os.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), srv.got.String()) // Ensure the contents match.
		assert.Equal(t, 2, len(handler.batches))     // Ensure there are 2 batches.
		assert.Equal(t, uint(32), srv.received)
		assert.Equal(t, uint(32), n) // Ensure there are 32 events (minus metadata).
	})
	t.Run("failure-with-rate-limit", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(9, 16))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		// 16 + 9 (1st sec) < 32 (total send)
		assert.Error(t, err)
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
		assert.Equal(t, uint(16), srv.received)  // Only the first batch is read
		assert.Equal(t, uint(16), n)
	})
	t.Run("burst-greater-than-bucket-error", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(10, 10))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, uint(0), srv.received)
		assert.Equal(t, n, uint(0))
	})
}

func TestHandlerWarmUp(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(rate.Inf, 0))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		// Warm up with more events than  saved.
		err := h.WarmUpServer(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Greater(t, srv.received, uint(0))
	})
	t.Run("cancel-before-warmup", func(t *testing.T) {
		h, srv := newHandler(t, "testdata", "python*.ndjson", rate.NewLimiter(rate.Inf, 0))
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cancel()
		err := h.WarmUpServer(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, srv.received, uint(0))
	})
}

func BenchmarkSendBatches(b *testing.B) {
	h, srv := newHandler(b, "testdata", "python*.ndjson", rate.NewLimiter(rate.Inf, 0))
	b.Cleanup(srv.close)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.SendBatches(ctx)
	}
}
