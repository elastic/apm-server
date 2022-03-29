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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newHandler(t *testing.T, dir, expr string) (*Handler, *mockServer) {
	t.Helper()
	ms := &mockServer{got: &bytes.Buffer{}}
	srv := httptest.NewServer(ms)
	ms.close = srv.Close
	transp := NewTransport(srv.Client(), srv.URL, "")
	h, err := New(expr, transp, os.DirFS(dir), 0)
	require.NoError(t, err)
	return h, ms
}

func TestHandlerNew(t *testing.T) {
	storage := os.DirFS("testdata")
	t.Run("success-matches-files", func(t *testing.T) {
		h, err := New(`python*.ndjson`, nil, storage, 0)
		require.NoError(t, err)
		assert.Greater(t, len(h.batches), 0)
	})
	t.Run("failure-matches-no-files", func(t *testing.T) {
		h, err := New(`go*.ndjson`, nil, storage, 0)
		require.EqualError(t, err, "eventhandler: glob matched no files, please specify a valid glob pattern")
		assert.Nil(t, h)
	})
	t.Run("failure-invalid-glob", func(t *testing.T) {
		h, err := New(``, nil, storage, 0)
		require.EqualError(t, err, "eventhandler: glob matched no files, please specify a valid glob pattern")
		assert.Nil(t, h)
	})
	t.Run("failure-rum-data", func(t *testing.T) {
		storage := os.DirFS(filepath.Join("..", "..", "..", "testdata", "intake-v3"))
		h, err := New(`*.ndjson`, nil, storage, 0)
		require.EqualError(t, err, "rum data support not implemented")
		assert.Nil(t, h)
	})
}

func TestHandlerSendBatches(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson")
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		b, err := ioutil.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), srv.got.String()) // Ensure the contents match.
		assert.Equal(t, uint(32), srv.received)
		assert.Equal(t, uint(32), n)             // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
	})
	t.Run("cancel-before-sendbatches", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", "python*.ndjson")
		t.Cleanup(srv.close)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, srv.received, uint(0))
		assert.Equal(t, n, uint(0))
	})
	t.Run("returns-error", func(t *testing.T) {
		handler, srv := newHandler(t, "testdata", `python*.ndjson`)
		// Close the server prematurely to force an error.
		srv.close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, n, uint(0))
	})
}

func TestHandlerWarmUp(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, srv := newHandler(t, "testdata", "python*.ndjson")
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// Warm up with more events than  saved.
		warmupEvents := uint(1000)
		err := h.WarmUpServer(ctx, warmupEvents)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, srv.received, warmupEvents)
	})
	t.Run("cancel-before-warmup", func(t *testing.T) {
		h, srv := newHandler(t, "testdata", "python*.ndjson")
		t.Cleanup(srv.close)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := h.WarmUpServer(ctx, 10)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, srv.received, uint(0))
	})
	t.Run("cancel-deadline-exceeded", func(t *testing.T) {
		h, srv := newHandler(t, "testdata", "python*.ndjson")
		t.Cleanup(srv.close)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		warmupEvents := uint(100000)
		err := h.WarmUpServer(ctx, warmupEvents)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Greater(t, srv.received, uint(0))
		// Assert that the sent events are less than the desired since the
		// context was cancelled on timeout.
		assert.Less(t, srv.received, warmupEvents)
	})
}
