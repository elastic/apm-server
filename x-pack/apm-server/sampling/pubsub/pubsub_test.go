// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

const (
	beatID = "beat_id"
)

var (
	dataStream = pubsub.DataStreamConfig{
		Type:      "traces",
		Dataset:   "sampled",
		Namespace: "testing",
	}
)

func TestPublishSampledTraceIDs(t *testing.T) {
	requestBodies := make(chan string)
	ms := newMockElasticsearchServer(t)
	ms.onBulk = func(r *http.Request) {
		select {
		case <-r.Context().Done():
		case requestBodies <- readBody(r):
		}
	}
	pub := newPubsub(t, ms.srv, time.Millisecond, time.Minute)

	input := make([]string, 20)
	for i := 0; i < len(input); i++ {
		input[i] = uuid.Must(uuid.NewV4()).String()
	}

	// Publish in a separate goroutine, as it may get blocked if we don't
	// service bulk requests.
	ids := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	var g errgroup.Group
	defer g.Wait()
	defer cancel()
	g.Go(func() error {
		return pub.PublishSampledTraceIDs(ctx, ids)
	})
	g.Go(func() error {
		for _, id := range input {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ids <- id:
			}
			time.Sleep(10 * time.Millisecond) // sleep to force new requests
		}
		return nil
	})

	var received []string
	deadlineTimer := time.NewTimer(10 * time.Second)
	for len(received) < len(input) {
		select {
		case <-deadlineTimer.C:
			t.Fatal("timed out waiting for events to be received by server")
		case body := <-requestBodies:
			d := json.NewDecoder(bytes.NewReader([]byte(body)))
			for {
				action := make(map[string]interface{})
				err := d.Decode(&action)
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)
				assert.Equal(t, map[string]interface{}{"create": map[string]interface{}{}}, action)

				doc := make(map[string]interface{})
				assert.NoError(t, d.Decode(&doc))
				assert.Contains(t, doc, "@timestamp")
				assert.Equal(t, map[string]interface{}{"id": beatID}, doc["observer"])
				assert.Equal(t, dataStream.Type, doc["data_stream.type"])
				assert.Equal(t, dataStream.Dataset, doc["data_stream.dataset"])
				assert.Equal(t, dataStream.Namespace, doc["data_stream.namespace"])

				trace := doc["trace"].(map[string]interface{})
				traceID := trace["id"].(string)
				received = append(received, traceID)
				delete(trace, "id")
				assert.Empty(t, trace) // no other fields in "trace"

				delete(doc, "@timestamp")
				delete(doc, "data_stream.type")
				delete(doc, "data_stream.dataset")
				delete(doc, "data_stream.namespace")
				delete(doc, "observer")
				delete(doc, "trace")

				assert.Empty(t, doc) // no other fields in doc
			}
		}
	}

	// The publisher uses an esutil.BulkIndexer, which may index items out
	// of order due to having multiple goroutines picking items off a queue.
	assert.ElementsMatch(t, input, received)
}

func TestSubscribeSampledTraceIDs(t *testing.T) {
	ms := newMockElasticsearchServer(t)
	ms.statsGlobalCheckpoint = 99

	assertSearchQueryFilterEqual := func(filter, body string) {
		expect := fmt.Sprintf(
			`{"query":{"bool":{"filter":%s,"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`,
			filter,
		)
		assert.Equal(t, expect, body)
	}

	var searchRequests int
	ms.onSearch = func(r *http.Request) {
		body := readBody(r)
		searchRequests++
		switch searchRequests {
		case 1:
			assertSearchQueryFilterEqual(`[{"range":{"_seq_no":{"lte":99}}}]`, body)
			ms.searchResults = []searchHit{
				newSearchHit(1, "trace_1"),
				newSearchHit(2, "trace_2"),
			}
		case 2:
			// The previous _search responded non-empty, and the greatest
			// _seq_no was not equal to the global checkpoint: _search again
			// after _seq_no 2.
			assertSearchQueryFilterEqual(`[{"range":{"_seq_no":{"lte":99}}}]`, body)
			ms.searchResults = []searchHit{
				newSearchHit(3, "trace_3"),
				newSearchHit(98, "trace_98"),
			}
		case 3:
			// Again the previous _search responded non-empty, and the greatest
			// _seq_no was not equal to the global checkpoint: _search again
			// after _seq_no 98. This time we respond with no hits, so the
			// subscriber goes back to sleep.
			assertSearchQueryFilterEqual(`[{"range":{"_seq_no":{"lte":99}}}]`, body)
			ms.searchResults = nil
		case 4:
			// The search now has an exclusive lower bound of the previously
			// observed maximum _seq_no. When the global checkpoint is observed,
			// the server stops issuing search requests and goes back to sleep.
			assertSearchQueryFilterEqual(`[{"range":{"_seq_no":{"lte":99}}},{"range":{"_seq_no":{"gt":98}}}]`, body)
			ms.searchResults = []searchHit{
				newSearchHit(99, "trace_99"),
			}
		case 5:
			// After advancing the global checkpoint, a new search will be made
			// with increased lower and upper bounds.
			assertSearchQueryFilterEqual(`[{"range":{"_seq_no":{"lte":100}}},{"range":{"_seq_no":{"gt":99}}}]`, body)
			ms.searchResults = []searchHit{
				newSearchHit(100, "trace_100"),
			}
		}
	}

	ids, positions, closeSubscriber := newSubscriber(t, ms.srv)
	assert.Equal(t, "trace_1", expectValue(t, ids))
	assert.Equal(t, "trace_2", expectValue(t, ids))
	assert.Equal(t, "trace_3", expectValue(t, ids))
	assert.Equal(t, "trace_98", expectValue(t, ids))
	assert.Equal(t, "trace_99", expectValue(t, ids))
	expectNone(t, ids)

	// Wait for the position to be reported. The position should be
	// non-zero, and when used should resume subscription without
	// returning already observed IDs.
	var pos pubsub.SubscriberPosition
	select {
	case pos = <-positions:
		assert.NotZero(t, pos)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for position to be reported")
	}

	// close first subscriber, create a new one initialised with position
	closeSubscriber()
	ids, positions, _ = newSubscriberPosition(t, ms.srv, pos)

	// Global checkpoint hasn't changed.
	expectNone(t, ids)

	// Advance global checkpoint, expect a new search and new position to be reported.
	ms.statsGlobalCheckpointMu.Lock()
	ms.statsGlobalCheckpoint = 100
	ms.statsGlobalCheckpointMu.Unlock()
	assert.Equal(t, "trace_100", expectValue(t, ids))
	select {
	case pos2 := <-positions:
		assert.NotEqual(t, pos, pos2)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for position to be reported")
	}
}

func TestSubscribeSampledTraceIDsErrors(t *testing.T) {
	statsRequests := make(chan struct{})
	firstStats := true
	m := newMockElasticsearchServer(t)
	m.searchStatusCode = http.StatusNotFound
	m.statsGlobalCheckpoint = 99
	m.onStats = func(r *http.Request) {
		select {
		case <-r.Context().Done():
		case statsRequests <- struct{}{}:
		}
		if firstStats {
			firstStats = false
			return
		}
		m.statsStatusCode = http.StatusInternalServerError
	}
	newSubscriber(t, m.srv)

	// Show that failed requests to Elasticsearch are not fatal, and
	// that the subscriber will retry.
	timeout := time.After(10 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-statsRequests:
		case <-timeout:
			t.Fatal("timed out waiting for _stats request")
		}
	}
}

func newSubscriber(t testing.TB, srv *httptest.Server) (<-chan string, <-chan pubsub.SubscriberPosition, context.CancelFunc) {
	return newSubscriberPosition(t, srv, pubsub.SubscriberPosition{})
}

func newSubscriberPosition(t testing.TB, srv *httptest.Server, pos pubsub.SubscriberPosition) (<-chan string, <-chan pubsub.SubscriberPosition, context.CancelFunc) {
	sub := newPubsub(t, srv, time.Minute, time.Millisecond)
	ids := make(chan string)
	positions := make(chan pubsub.SubscriberPosition)
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return sub.SubscribeSampledTraceIDs(ctx, pos, ids, positions)
	})
	cancelFunc := func() {
		cancel()
		g.Wait()
	}
	t.Cleanup(cancelFunc)
	return ids, positions, cancelFunc
}

func newPubsub(t testing.TB, srv *httptest.Server, flushInterval, searchInterval time.Duration) *pubsub.Pubsub {
	client, err := elasticsearch.NewClient(&elasticsearch.Config{
		Hosts: []string{srv.Listener.Addr().String()},
	})
	require.NoError(t, err)

	sub, err := pubsub.New(pubsub.Config{
		Client:         client,
		DataStream:     dataStream,
		BeatID:         beatID,
		FlushInterval:  flushInterval,
		SearchInterval: searchInterval,
	})
	require.NoError(t, err)
	return sub
}

type mockElasticsearchServer struct {
	srv *httptest.Server

	// statsGlobalCheckpoint is the shard seq_no global_checkpoint to respond with in
	// the _stats/get handler. If this is negative, the handler responds with status
	statsGlobalCheckpointMu sync.RWMutex
	statsGlobalCheckpoint   int

	// statsStatusCode is the status code that the _stats/get handler responds with.
	statsStatusCode int

	// searchResults is the search hits that the _search handler responds with.
	searchResults []searchHit

	// searchStatusCode is the status code that the _search handler responds with.
	searchStatusCode int

	// onStats is a function that is invoked whenever a _stats/get request is received.
	// This may be used to adjust the status code or global checkpoint that will be
	// returned.
	onStats func(r *http.Request)

	// onSearch is a function that is invoked whenever a _search request is received.
	// This may be used to check the search query, and adjust the search results that
	// will be returned.
	onSearch func(r *http.Request)

	// onBulk is a function that is invoked whenever a _bulk request is received.
	// This may be used to check the publication of sampled trace IDs.
	onBulk func(r *http.Request)
}

func newMockElasticsearchServer(t testing.TB) *mockElasticsearchServer {
	m := &mockElasticsearchServer{
		statsStatusCode:  http.StatusOK,
		searchStatusCode: http.StatusOK,
		onStats:          func(*http.Request) {},
		onSearch:         func(*http.Request) {},
		onBulk:           func(*http.Request) {},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			return
		}
		panic(fmt.Errorf("unexpected URL path: %s", r.URL.Path))
	})
	mux.HandleFunc("/"+dataStream.String()+"/_bulk", m.handleBulk)
	mux.HandleFunc("/"+dataStream.String()+"/_stats/get", m.handleStats)
	mux.HandleFunc("/index_name/_refresh", m.handleRefresh)
	mux.HandleFunc("/index_name/_search", m.handleSearch)

	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockElasticsearchServer) handleStats(w http.ResponseWriter, r *http.Request) {
	m.onStats(r)
	w.WriteHeader(m.statsStatusCode)
	if m.statsStatusCode != http.StatusOK {
		return
	}

	m.statsGlobalCheckpointMu.RLock()
	checkpoint := m.statsGlobalCheckpoint
	m.statsGlobalCheckpointMu.RUnlock()

	w.Write([]byte(fmt.Sprintf(`{
          "indices": {
	    "index_name": {
              "shards": {
	        "0": [{
		  "routing": {
		    "primary": true
		  },
		  "seq_no": {
		    "global_checkpoint": %d
		  }
		}]
	      }
	    }
	  }
	}`, checkpoint)))
}

func (m *mockElasticsearchServer) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Empty 200 OK response
}

func (m *mockElasticsearchServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	m.onSearch(r)
	w.WriteHeader(m.searchStatusCode)
	if m.searchStatusCode != http.StatusOK {
		return
	}
	var body struct {
		Hits struct {
			Hits []searchHit `json:"hits"`
		} `json:"hits"`
	}
	body.Hits.Hits = m.searchResults
	json.NewEncoder(w).Encode(body)
}

func (m *mockElasticsearchServer) handleBulk(w http.ResponseWriter, r *http.Request) {
	m.onBulk(r)
}

func expectValue(t testing.TB, ch <-chan string) string {
	t.Helper()
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for trace ID to be sent")
		panic("unreachable")
	case v, ok := <-ch:
		assert.True(t, ok)
		return v
	}
}

func expectNone(t testing.TB, ch <-chan string) {
	t.Helper()
	select {
	case <-time.After(100 * time.Millisecond):
	case v := <-ch:
		t.Errorf("unexpected send on channel: %q", v)
	}
}

func readBody(r *http.Request) string {
	var buf bytes.Buffer
	io.Copy(&buf, r.Body)
	return strings.TrimSpace(buf.String())
}

type searchHit struct {
	SeqNo  int64           `json:"_seq_no,omitempty"`
	Source traceIDDocument `json:"_source"`
	Sort   []int64         `json:"sort"`
}

func newSearchHit(seqNo int64, traceID string) searchHit {
	var source traceIDDocument
	source.Observer.ID = "another_beat_id"
	source.Trace.ID = traceID
	return searchHit{SeqNo: seqNo, Source: source, Sort: []int64{seqNo}}
}

type traceIDDocument struct {
	Observer struct {
		ID string `json:"id"`
	} `json:"observer"`

	Trace struct {
		ID string `json:"id"`
	} `json:"trace"`
}
