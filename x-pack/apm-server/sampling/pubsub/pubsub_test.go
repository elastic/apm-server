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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
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
	srv, requests := newRequestResponseWriterServer(t)
	pub := newPubsub(t, srv, time.Millisecond, time.Minute)

	var ids []string
	for i := 0; i < 20; i++ {
		ids = append(ids, uuid.Must(uuid.NewV4()).String())
	}

	// Publish in a separate goroutine, as it may get blocked if we don't
	// service bulk requests.
	go func() {
		for i := 0; i < len(ids); i += 2 {
			err := pub.PublishSampledTraceIDs(context.Background(), ids[i], ids[i+1])
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // sleep to force a new request
		}
	}()

	var received []string
	deadlineTimer := time.NewTimer(10 * time.Second)
	for len(received) < len(ids) {
		select {
		case <-deadlineTimer.C:
			t.Fatal("timed out waiting for events to be received by server")
		case rw := <-requests:
			require.Equal(t, fmt.Sprintf("/%s/_bulk", dataStream.String()), rw.URL.Path)

			body, err := ioutil.ReadAll(rw.Body)
			require.NoError(t, err)
			rw.Write("") // unblock client

			d := json.NewDecoder(bytes.NewReader(body))
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
	assert.ElementsMatch(t, ids, received)
}

func TestSubscribeSampledTraceIDs(t *testing.T) {
	srv, requests := newRequestResponseWriterServer(t)
	ids, _ := newSubscriber(t, srv)

	expectRequest(t, requests, "/traces-sampled-testing/_stats", "").Write(`{
          "indices": {
	    "index_name": {
              "shards": {
	        "0": [{
		  "routing": {
		    "primary": true
		  },
		  "seq_no": {
		    "global_checkpoint": 99
		  }
		}]
	      }
	    }
	  }
	}`)
	expectRequest(t, requests, "/index_name/_pit", "").Write(`{"id": "pit_id_1"}`)

	// _search: we respond with some results, returning a new PIT ID.
	expectRequest(t, requests, "/_search", `{"pit":{"id":"pit_id_1","keep_alive":"1m"},"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
	  "pit_id": "pit_id_2",
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 1,
		"_source": {"trace": {"id": "trace_1"}, "observer": {"id": "another_beat_id"}},
		"sort": [1]
	      },
	      {
	        "_seq_no": 2,
		"_source": {"trace": {"id": "trace_2"}, "observer": {"id": "another_beat_id"}},
		"sort": [2]
	      }
	    ]
	  }
	}`)

	assert.Equal(t, "trace_1", expectValue(t, ids))
	assert.Equal(t, "trace_2", expectValue(t, ids))

	// The previous _search responded non-empty, and the greatest _seq_no was not equal
	// to the global checkpoint: _search again after _seq_no 2.
	expectRequest(t, requests, "/_search", `{"pit":{"id":"pit_id_2","keep_alive":"1m"},"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[2],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
	  "pit_id": "pit_id_2",
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 3,
		"_source": {"trace": {"id": "trace_3"}, "observer": {"id": "another_beat_id"}},
		"sort": [3]
	      },
	      {
	        "_seq_no": 98,
		"_source": {"trace": {"id": "trace_98"}, "observer": {"id": "another_beat_id"}},
		"sort": [98]
	      }
	    ]
	  }
	}`)

	assert.Equal(t, "trace_3", expectValue(t, ids))
	assert.Equal(t, "trace_98", expectValue(t, ids))

	// Again the previous _search responded non-empty, and the greatest _seq_no was not equal
	// to the global checkpoint: _search again after _seq_no 98. This time we respond with no
	// hits, so the subscriber goes back to sleep.
	expectRequest(t, requests, "/_search", `{"pit":{"id":"pit_id_2","keep_alive":"1m"},"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[98],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(
		`{"hits":{"hits":[]}}`,
	)
	expectNone(t, ids)
	expectRequest(t, requests, "/_pit", `{"id":"pit_id_1"}`).Write("{}") // close PIT

	// _stats: respond with the same global checkpoint as before
	expectRequest(t, requests, "/traces-sampled-testing/_stats", "").Write(`{
          "indices": {
	    "index_name": {
              "shards": {
	        "0": [{
		  "routing": {
		    "primary": true
		  },
		  "seq_no": {
		    "global_checkpoint": 99
		  }
		}]
	      }
	    }
	  }
	}`)
	expectRequest(t, requests, "/index_name/_pit", "").Write(`{"id": "pit_id_3"}`)

	// The search now has an exclusive lower bound of the previously observed maximum _seq_no.
	// When the global checkpoint is observed, the server stops issuing search requests and
	// goes back to sleep.
	expectRequest(t, requests, "/_search", `{"pit":{"id":"pit_id_3","keep_alive":"1m"},"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}},{"range":{"_seq_no":{"gt":98}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
	  "pit_id": "pit_id_3",
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 99,
		"_source": {"trace": {"id": "trace_99"}, "observer": {"id": "another_beat_id"}},
		"sort": [99]
	      }
	    ]
	  }
	}`)
	assert.Equal(t, "trace_99", expectValue(t, ids))
	expectRequest(t, requests, "/_pit", `{"id":"pit_id_3"}`).Write("{}") // close PIT
}

func TestSubscribeSampledTraceIDsErrors(t *testing.T) {
	srv, requests := newRequestResponseWriterServer(t)
	newSubscriber(t, srv)

	expectRequest(t, requests, "/traces-sampled-testing/_stats", "").Write(`{
          "indices": {
	    "index_name": {
              "shards": {
	        "0": [{
		  "routing": {
		    "primary": true
		  },
		  "seq_no": {
		    "global_checkpoint": 99
		  }
		}]
	      }
	    }
	  }
	}`)
	expectRequest(t, requests, "/index_name/_pit", "").WriteStatus(404, "not found")
	expectRequest(t, requests, "/traces-sampled-testing/_stats", "").WriteStatus(500, "")
	expectRequest(t, requests, "/traces-sampled-testing/_stats", "").WriteStatus(500, "") // errors are not fatal
}

func newSubscriber(t testing.TB, srv *httptest.Server) (<-chan string, context.CancelFunc) {
	sub := newPubsub(t, srv, time.Minute, time.Millisecond)
	ids := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return sub.SubscribeSampledTraceIDs(ctx, ids)
	})
	cancelFunc := func() {
		cancel()
		g.Wait()
	}
	t.Cleanup(cancelFunc)
	return ids, cancelFunc
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

func newRequestResponseWriterServer(t testing.TB) (*httptest.Server, <-chan *requestResponseWriter) {
	requests := make(chan *requestResponseWriter)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rrw := &requestResponseWriter{
			Request: r,
			w:       w,
			done:    make(chan struct{}),
		}
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusRequestTimeout)
			return
		case requests <- rrw:
		}
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusRequestTimeout)
			return
		case <-rrw.done:
		}
	}))
	t.Cleanup(srv.Close)
	return srv, requests
}

type requestResponseWriter struct {
	*http.Request
	w    http.ResponseWriter
	done chan struct{}
}

func (w *requestResponseWriter) Write(body string) {
	w.WriteStatus(http.StatusOK, body)
}

func (w *requestResponseWriter) WriteStatus(statusCode int, body string) {
	w.w.WriteHeader(statusCode)
	w.w.Write([]byte(body))
	close(w.done)
}

func expectRequest(t testing.TB, ch <-chan *requestResponseWriter, path, body string) *requestResponseWriter {
	t.Helper()
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for request")
		panic("unreachable")
	case r, ok := <-ch:
		if assert.True(t, ok) {
			var buf bytes.Buffer
			io.Copy(&buf, r.Body)
			assert.Equal(t, path, r.URL.Path)
			assert.Equal(t, body, strings.TrimSpace(buf.String()))
			return r
		}
	}
	return nil
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
	case <-time.After(500 * time.Millisecond):
	case v := <-ch:
		t.Errorf("unexpected send on channel: %q", v)
	}
}
