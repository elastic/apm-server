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

package modelindexer_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2/apmtest"
	"go.elastic.co/fastjson"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/model/modelindexer"
	"github.com/elastic/apm-server/internal/model/modelindexer/modelindexertest"
)

func TestModelIndexer(t *testing.T) {
	var productOriginHeader string
	var bytesTotal int64
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		bytesTotal += r.ContentLength
		_, result := modelindexertest.DecodeBulkRequest(r)
		result.HasErrors = true
		// Respond with an error for the first two items, with one indicating
		// "too many requests". These will be recorded as failures in indexing
		// stats.
		for i := range result.Items {
			if i >= 2 {
				break
			}
			status := http.StatusInternalServerError
			if i == 1 {
				status = http.StatusTooManyRequests
			}
			for action, item := range result.Items[i] {
				item.Status = status
				result.Items[i][action] = item
			}
		}
		productOriginHeader = r.Header.Get("X-Elastic-Product-Origin")
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Minute})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	available := indexer.Stats().AvailableBulkRequests
	const N = 10
	for i := 0; i < N; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case <-time.After(10 * time.Millisecond):
			// Because the internal channel is buffered to increase performance,
			// the available indexer may not take events right away, loop until
			// the available bulk requests has been lowered.
			if indexer.Stats().AvailableBulkRequests < available {
				break loop
			}
		case <-timeout:
			t.Fatalf("timed out waiting for the active bulk indexer to pull from the available queue")
		}
	}
	// Indexer has not been flushed, there is one active bulk indexer.
	assert.Equal(t, modelindexer.Stats{Added: N, Active: N, AvailableBulkRequests: 24, ActiveBulkRequests: 1}, indexer.Stats())

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.NoError(t, err)
	stats := indexer.Stats()
	assert.Equal(t, modelindexer.Stats{
		Added:                 N,
		Active:                0,
		BulkRequests:          1,
		Failed:                2,
		Indexed:               N - 2,
		TooManyRequests:       1,
		AvailableBulkRequests: 25,
		BytesTotal:            bytesTotal,
	}, stats)
	assert.Equal(t, "observability", productOriginHeader)
}

func TestModelIndexerAvailableBulkIndexers(t *testing.T) {
	unblockRequests := make(chan struct{})
	receivedFlush := make(chan struct{})
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		receivedFlush <- struct{}{}
		// Wait until signaled to service requests
		<-unblockRequests
		_, result := modelindexertest.DecodeBulkRequest(r)
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Minute, FlushBytes: 1})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	const N = 25
	for i := 0; i < N; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for i := 0; i < N; i++ {
		select {
		case <-receivedFlush:
		case <-timeout.C:
			t.Fatalf("timed out waiting for %d, received %d", N, i)
		}
	}
	stats := indexer.Stats()
	// FlushBytes is set arbitrarily low, forcing a flush on each new
	// event. There should be no available bulk indexers.
	assert.Equal(t, modelindexer.Stats{Added: N, Active: N, AvailableBulkRequests: 0, ActiveBulkRequests: 1}, stats)

	close(unblockRequests)
	err = indexer.Close(context.Background())
	require.NoError(t, err)
	stats = indexer.Stats()
	stats.BytesTotal = 0 // Asserted elsewhere.
	assert.Equal(t, modelindexer.Stats{
		Added:                 N,
		BulkRequests:          N,
		Indexed:               N,
		AvailableBulkRequests: 25,
	}, stats)
}

func TestModelIndexerEncoding(t *testing.T) {
	var indexed [][]byte
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		var result elasticsearch.BulkIndexerResponse
		indexed, result = modelindexertest.DecodeBulkRequest(r)
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		FlushInterval: time.Minute,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{{
		Timestamp: time.Unix(123, 456789111).UTC(),
		DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		},
	}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.NoError(t, err)

	require.Len(t, indexed, 1)
	var decoded map[string]interface{}
	err = json.Unmarshal(indexed[0], &decoded)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"@timestamp":            "1970-01-01T00:02:03.456Z",
		"data_stream.type":      "logs",
		"data_stream.dataset":   "apm_server",
		"data_stream.namespace": "testing",
	}, decoded)
}

func TestModelIndexerCompressionLevel(t *testing.T) {
	var bytesTotal int64
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		bytesTotal += r.ContentLength
		_, result := modelindexertest.DecodeBulkRequest(r)
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		CompressionLevel: gzip.BestSpeed,
		FlushInterval:    time.Minute,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{{
		Timestamp: time.Unix(123, 456789111).UTC(),
		DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		},
	}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.NoError(t, err)
	stats := indexer.Stats()
	assert.Equal(t, modelindexer.Stats{
		Added:                 1,
		Active:                0,
		BulkRequests:          1,
		Failed:                0,
		Indexed:               1,
		TooManyRequests:       0,
		AvailableBulkRequests: 25,
		BytesTotal:            bytesTotal,
	}, stats)
}

func TestModelIndexerFlushInterval(t *testing.T) {
	requests := make(chan struct{}, 1)
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case requests <- struct{}{}:
		}
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		// Default flush bytes is 5MB
		FlushInterval: time.Millisecond,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	select {
	case <-requests:
		t.Fatal("unexpected request, no events buffered")
	case <-time.After(50 * time.Millisecond):
	}

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	select {
	case <-requests:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for request, flush interval elapsed")
	}
}

func TestModelIndexerFlushBytes(t *testing.T) {
	requests := make(chan struct{}, 1)
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case requests <- struct{}{}:
		default:
		}
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		FlushBytes: 1024,
		// Default flush interval is 30 seconds
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	select {
	case <-requests:
		t.Fatal("unexpected request, flush bytes not exceeded")
	case <-time.After(50 * time.Millisecond):
	}

	for i := 0; i < 100; i++ {
		err = indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	select {
	case <-requests:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for request, flush bytes exceeded")
	}
}

func TestModelIndexerServerError(t *testing.T) {
	var bytesTotal int64
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		bytesTotal += r.ContentLength
		w.WriteHeader(http.StatusInternalServerError)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Minute})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.EqualError(t, err, "flush failed: [500 Internal Server Error] ")
	stats := indexer.Stats()
	assert.Equal(t, modelindexer.Stats{
		Added:                 1,
		Active:                0,
		BulkRequests:          1,
		Failed:                1,
		AvailableBulkRequests: 25,
		BytesTotal:            bytesTotal,
	}, stats)
}

func TestModelIndexerServerErrorTooManyRequests(t *testing.T) {
	var bytesTotal int64
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Set the r.ContentLength rather than sum it since 429s will be
		// retried by the go-elasticsearch transport.
		bytesTotal = r.ContentLength
		w.WriteHeader(http.StatusTooManyRequests)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Minute})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.EqualError(t, err, "flush failed: [429 Too Many Requests] ")
	stats := indexer.Stats()
	assert.Equal(t, modelindexer.Stats{
		Added:                 1,
		Active:                0,
		BulkRequests:          1,
		Failed:                1,
		TooManyRequests:       1,
		AvailableBulkRequests: 25,
		BytesTotal:            bytesTotal,
	}, stats)
}

func TestModelIndexerLogRateLimit(t *testing.T) {
	logp.DevelopmentSetup(logp.ToObserverOutput())

	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		scanner := bufio.NewScanner(r.Body)
		result := elasticsearch.BulkIndexerResponse{HasErrors: true}
		for i := 0; scanner.Scan(); i++ {
			action := make(map[string]interface{})
			if err := json.NewDecoder(strings.NewReader(scanner.Text())).Decode(&action); err != nil {
				panic(err)
			}
			var actionType string
			for actionType = range action {
			}
			if !scanner.Scan() {
				panic("expected source")
			}
			item := esutil.BulkIndexerResponseItem{Status: http.StatusInternalServerError}
			item.Error.Type = "error_type"
			if i%2 == 0 {
				item.Error.Reason = "error_reason_even"
			} else {
				item.Error.Reason = "error_reason_odd"
			}
			result.Items = append(result.Items, map[string]esutil.BulkIndexerResponseItem{actionType: item})
		}
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushBytes: 500})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	for i := 0; i < 100; i++ {
		err = indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}
	err = indexer.Close(context.Background())
	assert.NoError(t, err)

	entries := logp.ObserverLogs().FilterMessageSnippet("failed to index event").TakeAll()
	require.Len(t, entries, 2)
	messages := []string{entries[0].Message, entries[1].Message}
	assert.ElementsMatch(t, []string{
		"failed to index event (error_type): error_reason_even",
		"failed to index event (error_type): error_reason_odd",
	}, messages)
}

func TestModelIndexerCloseFlushContext(t *testing.T) {
	srvctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-srvctx.Done():
		case <-r.Context().Done():
		}
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		FlushInterval: time.Millisecond,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	errch := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		errch <- indexer.Close(ctx)
	}()

	// Should be blocked in flush.
	select {
	case err := <-errch:
		t.Fatalf("unexpected return from indexer.Close: %s", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-errch:
		assert.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for flush to unblock")
	}
}

func TestModelIndexerCloseInterruptProcessBatch(t *testing.T) {
	srvctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-srvctx.Done():
		case <-r.Context().Done():
		}
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{
		// Set FlushBytes to 1 so a single event causes a flush.
		FlushBytes: 1,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	// Fill up all the bulk requests and the buffered channel.
	for n := indexer.Stats().AvailableBulkRequests + 100; n >= 0; n-- {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	// Call ProcessBatch again; this should block, as all bulk requests are
	// blocked and the buffered channel is full.
	encoded := make(chan struct{})
	processed := make(chan error, 1)
	processContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		batch := model.Batch{{
			Timestamp: time.Now(),
			DataStream: model.DataStream{
				Type:      "traces",
				Dataset:   "apm_server",
				Namespace: "testing",
			},
			Transaction: &model.Transaction{
				Custom: mapstr.M{
					"custom": marshalJSONFunc(func() ([]byte, error) {
						close(encoded)
						return []byte(`"encoded"`), nil
					}),
				},
			},
		}}
		processed <- indexer.ProcessBatch(processContext, &batch)
	}()

	// ProcessBatch should should encode the event, but then block.
	select {
	case <-encoded:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for event to be encoded by ProcessBatch")
	}
	select {
	case err := <-processed:
		t.Fatal("ProcessBatch returned unexpectedly", err)
	case <-time.After(50 * time.Millisecond):
	}

	// Close should block waiting for the enqueued events to be flushed, but
	// must honour the given context and not block forever.
	closed := make(chan error, 1)
	closeContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		closed <- indexer.Close(closeContext)
	}()
	select {
	case err := <-closed:
		t.Fatal("ProcessBatch returned unexpectedly", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-closed:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Close to return")
	}
	select {
	case err := <-processed:
		assert.ErrorIs(t, err, modelindexer.ErrClosed)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for ProcessBatch to return")
	}
}

type marshalJSONFunc func() ([]byte, error)

func (f marshalJSONFunc) MarshalJSON() ([]byte, error) {
	return f()
}

func TestModelIndexerFlushGoroutineStopped(t *testing.T) {
	bulkHandler := func(w http.ResponseWriter, r *http.Request) {}
	config := modelindexertest.NewMockElasticsearchClientConfig(t, bulkHandler)
	httpTransport, _ := elasticsearch.NewHTTPTransport(config)
	httpTransport.DisableKeepAlives = true // disable to avoid persistent conn goroutines
	client, _ := elasticsearch.NewClientParams(elasticsearch.ClientParams{
		Config:    config,
		Transport: httpTransport,
	})

	indexer, err := modelindexer.New(client, modelindexer.Config{FlushBytes: 1})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	before := runtime.NumGoroutine()
	for i := 0; i < 100; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	var after int
	deadline := time.Now().Add(10 * time.Second)
	for after > before && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		after = runtime.NumGoroutine()
	}
	assert.GreaterOrEqual(t, before, after, "Leaked %d goroutines", after-before)
}

func TestModelIndexerUnknownResponseFields(t *testing.T) {
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ingest_took":123}`))
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
		Type:      "logs",
		Dataset:   "apm_server",
		Namespace: "testing",
	}}}
	err = indexer.ProcessBatch(context.Background(), &batch)
	require.NoError(t, err)

	err = indexer.Close(context.Background())
	assert.NoError(t, err)
}

func TestModelIndexerCloseBusyIndexer(t *testing.T) {
	// This test ensures that all the channel items are consumed and indexed
	// when the indexer is closed.
	var bytesTotal int64
	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		bytesTotal += r.ContentLength
		_, result := modelindexertest.DecodeBulkRequest(r)
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { indexer.Close(context.Background()) })

	const N = 10000
	for i := 0; i < N; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err = indexer.ProcessBatch(context.Background(), &batch)
		assert.NoError(t, err)
	}

	assert.NoError(t, indexer.Close(context.Background()))

	assert.Equal(t, modelindexer.Stats{
		Added:                 N,
		Indexed:               N,
		BulkRequests:          1,
		BytesTotal:            bytesTotal,
		AvailableBulkRequests: 25,
		ActiveBulkRequests:    0}, indexer.Stats())
}

func TestModelIndexerTracing(t *testing.T) {
	testModelIndexerTracing(t, 200, "success")
	testModelIndexerTracing(t, 400, "failure")
}

func testModelIndexerTracing(t *testing.T, statusCode int, expectedOutcome string) {
	logp.DevelopmentSetup(logp.ToObserverOutput())

	client := modelindexertest.NewMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, result := modelindexertest.DecodeBulkRequest(r)
		json.NewEncoder(w).Encode(result)
	})

	tracer := apmtest.NewRecordingTracer()
	defer tracer.Close()
	indexer, err := modelindexer.New(client, modelindexer.Config{
		FlushInterval: time.Minute,
		Tracer:        tracer.Tracer,
	})
	require.NoError(t, err)
	defer indexer.Close(context.Background())

	const N = 100
	for i := 0; i < N; i++ {
		batch := model.Batch{model.APMEvent{Timestamp: time.Now(), DataStream: model.DataStream{
			Type:      "logs",
			Dataset:   "apm_server",
			Namespace: "testing",
		}}}
		err := indexer.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
	}

	// Closing the indexer flushes enqueued events.
	_ = indexer.Close(context.Background())

	tracer.Flush(nil)
	payloads := tracer.Payloads()
	require.Len(t, payloads.Transactions, 1)
	require.Len(t, payloads.Spans, 1)

	assert.Equal(t, expectedOutcome, payloads.Transactions[0].Outcome)
	assert.Equal(t, "output", payloads.Transactions[0].Type)
	assert.Equal(t, "flush", payloads.Transactions[0].Name)
	assert.Equal(t, "Elasticsearch: POST _bulk", payloads.Spans[0].Name)
	assert.Equal(t, "db", payloads.Spans[0].Type)
	assert.Equal(t, "elasticsearch", payloads.Spans[0].Subtype)

	entries := logp.ObserverLogs().TakeAll()
	assert.NotEmpty(t, entries)
	for _, entry := range entries {
		fields := entry.ContextMap()
		assert.Equal(t, fmt.Sprintf("%x", payloads.Transactions[0].ID), fields["transaction.id"])
		assert.Equal(t, fmt.Sprintf("%x", payloads.Transactions[0].TraceID), fields["trace.id"])
	}
}

func BenchmarkModelIndexer(b *testing.B) {
	b.Run("NoCompression", func(b *testing.B) {
		benchmarkModelIndexer(b, modelindexer.Config{
			CompressionLevel: gzip.NoCompression,
		})
	})
	b.Run("BestSpeed", func(b *testing.B) {
		benchmarkModelIndexer(b, modelindexer.Config{
			CompressionLevel: gzip.BestSpeed,
		})
	})
	b.Run("DefaultCompression", func(b *testing.B) {
		benchmarkModelIndexer(b, modelindexer.Config{
			CompressionLevel: gzip.DefaultCompression,
		})
	})
	b.Run("BestCompression", func(b *testing.B) {
		benchmarkModelIndexer(b, modelindexer.Config{
			CompressionLevel: gzip.BestCompression,
		})
	})
}

func benchmarkModelIndexer(b *testing.B, cfg modelindexer.Config) {
	var indexed int64
	client := modelindexertest.NewMockElasticsearchClient(b, func(w http.ResponseWriter, r *http.Request) {
		body := r.Body
		switch r.Header.Get("Content-Encoding") {
		case "gzip":
			r, err := gzip.NewReader(body)
			if err != nil {
				panic(err)
			}
			defer r.Close()
			body = r
		}

		var n int64
		var jsonw fastjson.Writer
		jsonw.RawString(`{"items":[`)
		first := true
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			// Action is always "create", skip decoding to avoid
			// inflating allocations in benchmark.
			if !scanner.Scan() {
				panic("expected source")
			}
			if first {
				first = false
			} else {
				jsonw.RawByte(',')
			}
			jsonw.RawString(`{"create":{"status":201}}`)
			n++
		}
		jsonw.RawString(`]}`)
		w.Write(jsonw.Bytes())
		atomic.AddInt64(&indexed, n)
	})
	cfg.FlushInterval = time.Second
	indexer, err := modelindexer.New(client, cfg)
	require.NoError(b, err)
	defer indexer.Close(context.Background())

	b.RunParallel(func(pb *testing.PB) {
		batch := model.Batch{
			model.APMEvent{
				Processor:   model.TransactionProcessor,
				Transaction: &model.Transaction{},
			},
		}
		for pb.Next() {
			batch[0].Timestamp = time.Now()
			batch[0].Transaction.ID = uuid.Must(uuid.NewV4()).String()
			if err := indexer.ProcessBatch(context.Background(), &batch); err != nil {
				b.Fatal(err)
			}
		}
	})
	// Closing the indexer flushes enqueued events.
	if err := indexer.Close(context.Background()); err != nil {
		b.Fatal(err)
	}
	assert.Equal(b, int64(b.N), indexed)
}
