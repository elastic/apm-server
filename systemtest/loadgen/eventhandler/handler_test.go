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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type mockServer struct {
	got          *bytes.Buffer
	close        func()
	received     int
	requestCount int
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
	t.requestCount++
	w.WriteHeader(http.StatusAccepted)
}

type newHandlerOption func(*Config)

func newHandler(tb testing.TB, opts ...newHandlerOption) (*Handler, *mockServer) {
	tb.Helper()

	ms := &mockServer{got: &bytes.Buffer{}}
	srv := httptest.NewServer(ms)
	ms.close = srv.Close
	transp := NewTransport(srv.Client(), srv.URL, "", "", nil)

	config := Config{
		Path:      "*.ndjson",
		Transport: transp,
		Storage:   os.DirFS("testdata"),
		Limiter:   rate.NewLimiter(rate.Inf, 0),
	}
	for _, opt := range opts {
		opt(&config)
	}

	h, err := New(config)
	require.NoError(tb, err)
	tb.Cleanup(srv.Close)

	return h, ms
}

func withStorage(fs fs.FS) newHandlerOption {
	return func(config *Config) {
		config.Storage = fs
	}
}

func withRateLimiter(l *rate.Limiter) newHandlerOption {
	return func(config *Config) {
		config.Limiter = l
	}
}

func withRewriteTimestamps(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteTimestamps = rewrite
	}
}

func withRewriteIDs(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteIDs = rewrite
	}
}

func withRand(rand *rand.Rand) newHandlerOption {
	return func(config *Config) {
		config.Rand = rand
	}
}

func TestHandlerNew(t *testing.T) {
	storage := os.DirFS("testdata")
	t.Run("success-matches-files", func(t *testing.T) {
		h, err := New(Config{
			Path:      `*.ndjson`,
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
		handler, srv := newHandler(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		b, err := os.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), srv.got.String()) // Ensure the contents match.
		assert.Equal(t, 32, srv.received)
		assert.Equal(t, 32, n)                   // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
	})
	t.Run("cancel-before-sendbatches", func(t *testing.T) {
		handler, srv := newHandler(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, 0, srv.received)
		assert.Equal(t, 0, n)
	})
	t.Run("returns-error", func(t *testing.T) {
		handler, srv := newHandler(t)
		// Close the server prematurely to force an error.
		srv.close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, 0, n)
	})
	t.Run("success-with-rate-limit", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(9, 16)))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		// 16 + 9 (1st sec) + 9 (2nd sec) > 32 (total send)
		assert.NoError(t, err)

		b, err := os.ReadFile(filepath.Join("testdata", "python-test.ndjson"))
		assert.NoError(t, err)

		assert.Equal(t, string(b), srv.got.String()) // Ensure the contents match.
		assert.Equal(t, 2, len(handler.batches))     // Ensure there are 2 batches.
		assert.Equal(t, 32, srv.received)
		assert.Equal(t, 32, n) // Ensure there are 32 events (minus metadata).
	})
	t.Run("failure-with-rate-limit", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(9, 16)))
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		// 16 + 9 (1st sec) < 32 (total send)
		assert.Error(t, err)
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
		assert.Equal(t, 16, srv.received)        // Only the first batch is read
		assert.Equal(t, 16, n)
	})
	t.Run("success-with-batch-bigger-than-burst", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(32, 8))) // rate=8/0.25s
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		// the payload is not equal as two metadata are added
		assert.Equal(t, 32, srv.received)
		assert.Equal(t, 32, n)                   // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
		assert.Equal(t, 4, srv.requestCount)     // a batch split into 2 requests.
	})
}

func TestHandlerSendBatchesRewriteTimestamps(t *testing.T) {
	t0 := time.Unix(123, 0)
	t1 := time.Unix(456, 78910)

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"transaction":{"timestamp":%d}}
{"span":{"timestamp":%q}}
{"error":{"timestamp":"invalid"}}
{"error":{"timestamp":-123}}
{"metricset":{}}
`[1:], t0.UnixMicro(), t1.Format(time.RFC3339Nano))

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			before := time.Now()
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteTimestamps(rewrite),
			)
			_, err := handler.SendBatches(context.Background())
			assert.NoError(t, err)

			if !rewrite {
				// Original payload should be sent as-is.
				assert.Equal(t, originalPayload, srv.got.String())
				return
			}

			d := json.NewDecoder(bytes.NewReader(srv.got.Bytes()))
			type object map[string]struct {
				Timestamp interface{} `json:"timestamp"`
			}
			var timestamps []interface{}
			for {
				var object object
				err := d.Decode(&object)
				if errors.Is(err, io.EOF) {
					break
				}
				require.NoError(t, err)
				for objtype, fields := range object {
					if objtype == "metadata" {
						break
					}
					timestamps = append(timestamps, fields.Timestamp)
				}
			}
			require.Len(t, timestamps, 5)

			assert.NotNil(t, timestamps[0]) // valid number, rewritten
			assert.IsType(t, "", timestamps[0])
			assert.NotNil(t, timestamps[1]) // valid string, rewritten
			assert.IsType(t, "", timestamps[1])
			assert.Equal(t, "invalid", timestamps[2])     // invalid, unmodified
			assert.Equal(t, float64(-123), timestamps[3]) // invalid number, unmodified
			assert.Nil(t, timestamps[4])                  // unspecified, unmodified

			t0rewritten, _ := time.Parse(time.RFC3339Nano, timestamps[0].(string))
			t1rewritten, _ := time.Parse(time.RFC3339Nano, timestamps[1].(string))
			assert.False(t, before.After(t0rewritten))
			assert.Equal(t, t1.Sub(t0), t1rewritten.Sub(t0rewritten))
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteIDs(t *testing.T) {
	traceID := "aaa111f"
	transactionID := "bbb222"
	spanID := "ccc333"
	errorID := "ffffffffeeeeeeee"

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"transaction":{"id":%q,"trace_id":%q}}
{"span":{"id":%q,"parent_id":%q,"trace_id":%q,"transaction_id":%q}}
{"metricset":{}}
{"error":{"id":%q}}
`[1:], transactionID, traceID, spanID, transactionID, traceID, transactionID, errorID)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteIDs(rewrite),
				withRand(rand.New(rand.NewSource(0))), // known seed
			)
			_, err := handler.SendBatches(context.Background())
			assert.NoError(t, err)

			if !rewrite {
				// Original payload should be sent as-is.
				assert.Equal(t, originalPayload, srv.got.String())
				return
			}

			d := json.NewDecoder(bytes.NewReader(srv.got.Bytes()))
			type object map[string]struct {
				ID            string `json:"id"`
				ParentID      string `json:"parent_id"`
				SpanID        string `json:"span_id"`
				TraceID       string `json:"trace_id"`
				TransactionID string `json:"transaction_id"`
			}
			var objects []object
			for {
				var object object
				err := d.Decode(&object)
				if errors.Is(err, io.EOF) {
					break
				}
				require.NoError(t, err)
				objects = append(objects, object)
			}

			assert.Equal(t, []object{
				{"metadata": {}},
				{"transaction": {ID: "ac3de0", TraceID: "bd2ed30"}},
				{"span": {ID: "db4cf1", ParentID: "ac3de0", TransactionID: "ac3de0", TraceID: "bd2ed30"}},
				{"metricset": {}},
				{"error": {ID: "e8703d0042c137ae"}},
			}, objects)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerWarmUp(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, srv := newHandler(t)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		// Warm up with more events than  saved.
		err := h.WarmUpServer(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Greater(t, srv.received, 0)
	})
	t.Run("cancel-before-warmup", func(t *testing.T) {
		h, srv := newHandler(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cancel()
		err := h.WarmUpServer(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, srv.received)
	})
}

func BenchmarkSendBatches(b *testing.B) {
	run := func(b *testing.B, opts ...newHandlerOption) {
		b.Helper()
		h, _ := newHandler(b, opts...)
		ctx := context.Background()
		b.ResetTimer()

		before := time.Now()
		var eventsSent int
		for i := 0; i < b.N; i++ {
			n, err := h.SendBatches(ctx)
			if err != nil {
				b.Fatal(err)
			}
			eventsSent += n
		}
		elapsed := time.Since(before)
		b.ReportMetric(float64(eventsSent)/elapsed.Seconds(), "events/sec")
	}
	b.Run("base", func(b *testing.B) {
		run(b)
	})
	b.Run("rewrite_timestamps", func(b *testing.B) {
		run(b, withRewriteTimestamps(true))
	})
	b.Run("rewrite_ids", func(b *testing.B) {
		run(b, withRewriteIDs(true))
	})
}
