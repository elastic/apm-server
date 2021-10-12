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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/go-elasticsearch/v7/esutil"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelindexer"
)

func TestModelIndexer(t *testing.T) {
	var indexed int64
	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		scanner := bufio.NewScanner(r.Body)
		var result elasticsearch.BulkIndexerResponse
		for scanner.Scan() {
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

			item := esutil.BulkIndexerResponseItem{Status: http.StatusCreated}
			if len(result.Items) == 0 {
				// Respond with an error for the first item. This will be recorded
				// as a failure in indexing stats.
				result.HasErrors = true
				item.Status = http.StatusInternalServerError
			}
			result.Items = append(result.Items, map[string]esutil.BulkIndexerResponseItem{actionType: item})

			if scanner.Scan() && scanner.Text() != "" {
				// Both the libbeat event encoder and bulk indexer add an empty line.
				panic("expected empty line")
			}
		}
		atomic.AddInt64(&indexed, int64(len(result.Items)))
		json.NewEncoder(w).Encode(result)
	})
	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Minute})
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
	assert.Equal(t, modelindexer.Stats{Added: N, Active: N}, indexer.Stats())

	// Closing the indexer flushes enqueued events.
	err = indexer.Close(context.Background())
	require.NoError(t, err)
	assert.Equal(t, modelindexer.Stats{
		Added:  N,
		Active: 0,
		Failed: 1,
	}, indexer.Stats())
}

func TestModelIndexerFlushInterval(t *testing.T) {
	requests := make(chan struct{}, 1)
	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	assert.Equal(t, modelindexer.Stats{
		Added:  1,
		Active: 0,
		Failed: 1,
	}, indexer.Stats())
}

func TestModelIndexerLogRateLimit(t *testing.T) {
	logp.DevelopmentSetup(logp.ToObserverOutput())

	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
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
			if scanner.Scan() && scanner.Text() != "" {
				// Both the libbeat event encoder and bulk indexer add an empty line.
				panic("expected empty line")
			}
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

	entries := logp.ObserverLogs().TakeAll()
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
	client := newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-srvctx.Done():
		case <-r.Context().Done():
		}
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

func BenchmarkModelIndexer(b *testing.B) {
	var indexed int64
	client := newMockElasticsearchClient(b, func(w http.ResponseWriter, r *http.Request) {
		scanner := bufio.NewScanner(r.Body)
		var n int64
		for scanner.Scan() {
			if scanner.Scan() {
				n++
			}
			if scanner.Scan() && scanner.Text() != "" {
				panic("expected empty line")
			}
		}
		atomic.AddInt64(&indexed, n)
		fmt.Fprintln(w, "{}")
	})

	indexer, err := modelindexer.New(client, modelindexer.Config{FlushInterval: time.Second})
	require.NoError(b, err)
	defer indexer.Close(context.Background())

	batch := model.Batch{
		model.APMEvent{
			Processor: model.TransactionProcessor,
			Timestamp: time.Now(),
		},
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
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

func newMockElasticsearchClient(t testing.TB, bulkHandler http.HandlerFunc) elasticsearch.Client {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	mux.Handle("/_bulk", bulkHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	config := elasticsearch.DefaultConfig()
	config.Hosts = elasticsearch.Hosts{srv.URL}
	client, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	return client
}
