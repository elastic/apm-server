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
	"sync/atomic"
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
	received     atomic.Int64
	requestCount atomic.Int64
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
			t.received.Add(1)
		}
		t.got.Write(line)
		t.got.Write([]byte("\n"))
	}
	t.requestCount.Add(1)
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

func withRewriteServiceNames(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteServiceNames = rewrite
	}
}

func withRewriteServiceNodeNames(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteServiceNodeNames = rewrite
	}
}

func withRewriteServiceTargetNames(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteServiceTargetNames = rewrite
	}
}

func withRewriteTransactionNames(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteTransactionNames = rewrite
	}
}

func withRewriteTransactionTypes(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteTransactionTypes = rewrite
	}
}

func withRewriteSpanNames(rewrite bool) newHandlerOption {
	return func(config *Config) {
		config.RewriteSpanNames = rewrite
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
		assert.Equal(t, int64(32), srv.received.Load())
		assert.Equal(t, 32, n)                   // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches)) // Ensure there are 2 batches.
	})
	t.Run("cancel-before-sendbatches", func(t *testing.T) {
		handler, srv := newHandler(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		n, err := handler.SendBatches(ctx)
		assert.Error(t, err)
		assert.Equal(t, int64(0), srv.received.Load())
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
		assert.Equal(t, int64(32), srv.received.Load())
		assert.Equal(t, 32, n) // Ensure there are 32 events (minus metadata).
	})
	t.Run("failure-with-rate-limit", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(9, 16)))
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		// 16 + 9 (1st sec) < 32 (total send)
		assert.Error(t, err)
		assert.Equal(t, 2, len(handler.batches))        // Ensure there are 2 batches.
		assert.Equal(t, int64(16), srv.received.Load()) // Only the first batch is read
		assert.Equal(t, 16, n)
	})
	t.Run("success-with-batch-bigger-than-burst", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(32, 8))) // rate=8/0.25s
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, err := handler.SendBatches(ctx)
		assert.NoError(t, err)

		// the payload is not equal as two metadata are added
		assert.Equal(t, int64(32), srv.received.Load())
		assert.Equal(t, 32, n)                             // Ensure there are 32 events (minus metadata).
		assert.Equal(t, 2, len(handler.batches))           // Ensure there are 2 batches.
		assert.Equal(t, int64(4), srv.requestCount.Load()) // a batch split into 2 requests.
	})
	t.Run("success-queue-when-batch-size-is-smaller-than-burst", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(6, 12))) // event rate = 12/2sec
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, _ := handler.SendBatches(ctx)
		// 12 (1st sec) sent, wait for 2 sec to send events
		// cancelling at 2nd sec shows queued batches
		assert.Equal(t, 2, len(handler.batches))        // Ensure there are 2 batches.
		assert.Equal(t, int64(12), srv.received.Load()) // no events sent until next interval (only first burst)
		assert.Equal(t, 12, n)
	})
	t.Run("success-dequeue-events-and-send-at-next-interval", func(t *testing.T) {
		handler, srv := newHandler(t, withRateLimiter(rate.NewLimiter(6, 12))) // event rate = 12/2sec
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		n, _ := handler.SendBatches(ctx)
		// 12 (1st sec) + 12 (3rd sec)
		assert.Equal(t, int64(24), srv.received.Load()) // Ensure the events are sent after 2 seconds
		assert.Equal(t, 24, n)
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

func TestHandlerSendBatchesRewriteServiceNames(t *testing.T) {
	serviceName := "Name_123"

	originalPayload := fmt.Sprintf(`
{"metadata":{"service":{"name":"%s"}}}
{"transaction":{}}
`[1:], serviceName)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteServiceNames(rewrite),
				withRand(rand.New(rand.NewSource(123456))), // known seed
			)
			_, err := handler.SendBatches(context.Background())
			assert.NoError(t, err)

			if !rewrite {
				// Original payload should be sent as-is.
				assert.Equal(t, originalPayload, srv.got.String())
				return
			}

			d := json.NewDecoder(bytes.NewReader(srv.got.Bytes()))
			var object struct {
				Metadata struct {
					Service struct {
						Name string `json:"name"`
					} `json:"service"`
				} `json:"metadata"`
			}

			err = d.Decode(&object)
			require.NoError(t, err)
			assert.Equal(t, "Swfc_801", object.Metadata.Service.Name)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteServiceNodeNames(t *testing.T) {
	serviceNodeName := "Name_123_Instance1"

	originalPayload := fmt.Sprintf(`
{"metadata":{"service":{"node":{"configured_name":"%s"}}}}
{"transaction":{}}
`[1:], serviceNodeName)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteServiceNodeNames(rewrite),
				withRand(rand.New(rand.NewSource(123))), // known seed
			)
			_, err := handler.SendBatches(context.Background())
			assert.NoError(t, err)

			if !rewrite {
				// Original payload should be sent as-is.
				assert.Equal(t, originalPayload, srv.got.String())
				return
			}

			d := json.NewDecoder(bytes.NewReader(srv.got.Bytes()))
			var object struct {
				Metadata struct {
					Service struct {
						Node struct {
							Name string `json:"configured_name"`
						} `json:"node"`
					} `json:"service"`
				} `json:"metadata"`
			}

			err = d.Decode(&object)
			require.NoError(t, err)
			assert.Equal(t, "Rjju_921_Lrapcsti9", object.Metadata.Service.Node.Name)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteServiceTargetNames(t *testing.T) {
	serviceTargetName := "Name_123"

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"span":{"context":{"service":{"target":{"type":"foo","name":%q}}}}}
`[1:], serviceTargetName)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteServiceTargetNames(rewrite),
				withRand(rand.New(rand.NewSource(123456))), // known seed
			)
			_, err := handler.SendBatches(context.Background())
			assert.NoError(t, err)

			if !rewrite {
				// Original payload should be sent as-is.
				assert.Equal(t, originalPayload, srv.got.String())
				return
			}

			d := json.NewDecoder(bytes.NewReader(srv.got.Bytes()))
			var object struct {
				Span struct {
					Context struct {
						Service struct {
							Target struct {
								Name string `json:"name"`
								Type string `json:"type"`
							} `json:"target"`
						} `json:"service"`
					} `json:"context"`
				} `json:"span"`
			}

			d.Decode(&object) // skip metadata

			err = d.Decode(&object)
			require.NoError(t, err)
			assert.Equal(t, "Swfc_801", object.Span.Context.Service.Target.Name)
			assert.Equal(t, "foo", object.Span.Context.Service.Target.Type)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteSpanNames(t *testing.T) {
	transactionName := "Transaction_Name_123"
	spanName := "Span_abc123_Name"

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"transaction":{"name":%q}}
{"span":{"name":%q}}
`[1:], transactionName, spanName)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteSpanNames(rewrite),
				withRand(rand.New(rand.NewSource(123456))), // known seed
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
				Name string `json:"name"`
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
				{"transaction": {Name: "Transaction_Name_123"}},
				{"span": {Name: "Swfc_cef201_Nfmk"}},
			}, objects)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteTransactionNames(t *testing.T) {
	transactionName := "Transaction_Name_123"
	spanName := "Span_abc123_Name"

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"transaction":{"name":%q}}
{"span":{"name":%q}}
`[1:], transactionName, spanName)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteTransactionNames(rewrite),
				withRand(rand.New(rand.NewSource(123456))), // known seed
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
				Name string `json:"name"`
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
				{"transaction": {Name: "Swfcucefmkl_Nfmk_959"}},
				{"span": {Name: "Span_abc123_Name"}},
			}, objects)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerSendBatchesRewriteTransactionTypes(t *testing.T) {
	transactionType := "Transaction_Type_123"

	originalPayload := fmt.Sprintf(`
{"metadata":{}}
{"transaction":{"type":%q}}
`[1:], transactionType)

	fs := fstest.MapFS{"foo.ndjson": {Data: []byte(originalPayload)}}

	run := func(t *testing.T, rewrite bool) {
		t.Helper()
		t.Run(strconv.FormatBool(rewrite), func(t *testing.T) {
			handler, srv := newHandler(t,
				withStorage(fs),
				withRewriteTransactionTypes(rewrite),
				withRand(rand.New(rand.NewSource(123456))), // known seed
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
				Type string `json:"type"`
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
				{"transaction": {Type: "Swfcucefmkl_Nfmk_959"}},
			}, objects)
		})
	}
	run(t, false)
	run(t, true)
}

func TestHandlerInLoop(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h, srv := newHandler(t)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		// Warm up with more events than  saved.
		err := h.SendBatchesInLoop(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Greater(t, srv.received.Load(), int64(0))
	})
	t.Run("cancel-before-warmup", func(t *testing.T) {
		h, srv := newHandler(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cancel()
		err := h.SendBatchesInLoop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int64(0), srv.received.Load())
	})

	t.Run("success-with-burst-bigger-than-batches", func(t *testing.T) {
		// the total number of events in batches are smaller than burst
		// in this case it should call multiple sendBatches to meet the target burst
		h, srv := newHandler(t, withRateLimiter(rate.NewLimiter(40*rate.Every(time.Second), 40)))
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := h.SendBatchesInLoop(ctx)

		assert.Error(t, err)
		assert.Equal(t, int64(80), srv.received.Load())
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
	b.Run("rewrite_service_names", func(b *testing.B) {
		run(b, withRewriteServiceNames(true))
	})
	b.Run("rewrite_service_node_names", func(b *testing.B) {
		run(b, withRewriteServiceNodeNames(true))
	})
	b.Run("rewrite_service_target_names", func(b *testing.B) {
		run(b, withRewriteServiceTargetNames(true))
	})
	b.Run("rewrite_transaction_names", func(b *testing.B) {
		run(b, withRewriteTransactionNames(true))
	})
	b.Run("rewrite_transaction_types", func(b *testing.B) {
		run(b, withRewriteTransactionTypes(true))
	})
	b.Run("rewrite_span_names", func(b *testing.B) {
		run(b, withRewriteSpanNames(true))
	})
}
