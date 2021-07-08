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
	select {
	case <-deadlineTimer.C:
		t.Fatal("timed out waiting for events to be received by server")
	case rw := <-requests:
		// burn initial request to index
		require.Equal(t, "/", rw.URL.Path)
		rw.Write("") // unblock client
	}
	for len(received) < len(input) {
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
	assert.ElementsMatch(t, input, received)
}

func TestSubscribeSampledTraceIDs(t *testing.T) {
	srv, requests := newRequestResponseWriterServer(t)
	ids, positions, cancel := newSubscriber(t, srv)

	expectRequest(t, requests, "/", "").Write("")
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").Write(`{
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

	// _refresh
	expectRequest(t, requests, "/index_name/_refresh", "").Write("")

	// _search: we respond with some results.
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
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
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
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
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(
		`{"hits":{"hits":[]}}`,
	)
	expectNone(t, ids)

	// _stats: respond with the same global checkpoint as before
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").Write(`{
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

	// _refresh
	expectRequest(t, requests, "/index_name/_refresh", "").Write("")

	// The search now has an exclusive lower bound of the previously observed maximum _seq_no.
	// When the global checkpoint is observed, the server stops issuing search requests and
	// goes back to sleep.
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}},{"range":{"_seq_no":{"gt":98}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
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

	// Wait for the position to be reported. The position is only reported
	// between searches, so for the test we need to unblock search requests
	// until a position is received.
	//
	// The returned position should be non-zero, and when used should
	// resume subscription without returning already observed IDs.
	var pos pubsub.SubscriberPosition
	var gotPosition bool
	for !gotPosition {
		select {
		case pos = <-positions:
			gotPosition = true
		case r := <-requests:
			r.WriteStatus(500, "")
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for subscriber position")
		}
	}
	assert.NotZero(t, pos)
	cancel() // close first subscriber
	ids, positions, cancel = newSubscriberPosition(t, srv, pos)
	defer cancel()

	// Respond initially with the same _seq_no as before, indicating there
	// have been no new docs since the position was recorded.
	expectRequest(t, requests, "/", "").Write("")
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").Write(`{
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

	// No changes, so after the interval elapses we'll check again. Now there
	// has been a new document written.
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").Write(`{
          "indices": {
	    "index_name": {
              "shards": {
	        "0": [{
		  "routing": {
		    "primary": true
		  },
		  "seq_no": {
		    "global_checkpoint": 100
		  }
		}]
	      }
	    }
	  }
	}`)

	// _refresh
	expectRequest(t, requests, "/index_name/_refresh", "").Write("")

	// _search: we respond with some results.
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":100}}},{"range":{"_seq_no":{"gt":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).Write(`{
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 100,
		"_source": {"trace": {"id": "trace_100"}, "observer": {"id": "another_beat_id"}},
		"sort": [100]
	      }
	    ]
	  }
	}`)
	assert.Equal(t, "trace_100", expectValue(t, ids))

	var pos2 pubsub.SubscriberPosition
	gotPosition = false
	for !gotPosition {
		select {
		case pos2 = <-positions:
			gotPosition = true
		case r := <-requests:
			r.WriteStatus(500, "")
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for subscriber position")
		}
	}
	assert.NotEqual(t, pos, pos2)
}

func TestSubscribeSampledTraceIDsErrors(t *testing.T) {
	srv, requests := newRequestResponseWriterServer(t)
	newSubscriber(t, srv)

	expectRequest(t, requests, "/", "").Write("")
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").Write(`{
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
	expectRequest(t, requests, "/index_name/_refresh", "").Write("")
	expectRequest(t, requests, "/index_name/_search", `{"query":{"bool":{"filter":[{"range":{"_seq_no":{"lte":99}}}],"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}],"track_total_hits":false}`).WriteStatus(404, "")
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").WriteStatus(500, "")
	expectRequest(t, requests, "/traces-sampled-testing/_stats/get", "").WriteStatus(500, "") // errors are not fatal
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

func newRequestResponseWriterServer(t testing.TB) (*httptest.Server, <-chan *requestResponseWriter) {
	requests := make(chan *requestResponseWriter)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rrw := &requestResponseWriter{
			Request: r,
			done:    make(chan response),
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
		case response := <-rrw.done:
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			w.WriteHeader(response.statusCode)
			w.Write([]byte(response.body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, requests
}

type requestResponseWriter struct {
	*http.Request
	done chan response
}

type response struct {
	statusCode int
	body       string
}

func (w *requestResponseWriter) Write(body string) {
	w.WriteStatus(http.StatusOK, body)
}

func (w *requestResponseWriter) WriteStatus(statusCode int, body string) {
	w.done <- response{statusCode, body}
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
