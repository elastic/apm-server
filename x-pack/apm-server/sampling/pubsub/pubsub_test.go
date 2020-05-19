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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/go-elasticsearch/v7"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

func TestPublishSampledTraceIDs(t *testing.T) {
	const (
		indexName = "trace-ids"
		beatID    = "beat_id"
	)

	requests := make(chan *http.Request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r.Body); err != nil {
			panic(err)
		}
		r.Body = ioutil.NopCloser(&buf)

		select {
		case <-r.Context().Done():
		case requests <- r:
		}
	}))
	defer srv.Close()

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{srv.URL},
	})
	require.NoError(t, err)

	pub, err := pubsub.New(pubsub.Config{
		Client:         client,
		Index:          indexName,
		BeatID:         beatID,
		FlushInterval:  time.Millisecond,
		SearchInterval: time.Minute,
	})
	require.NoError(t, err)

	var ids []string
	for i := 0; i < 20; i++ {
		ids = append(ids, fmt.Sprintf("trace_%d", len(ids)))
	}

	// Publish in a separate goroutine, as it may get blocked if we don't
	// service bulk requests.
	go func() {
		for i := 0; i < len(ids); i += 2 {
			err = pub.PublishSampledTraceIDs(context.Background(), ids[i], ids[i+1])
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
		case req := <-requests:
			require.Equal(t, fmt.Sprintf("/%s/_bulk", indexName), req.URL.Path)

			d := json.NewDecoder(req.Body)
			for {
				action := make(map[string]interface{})
				err := d.Decode(&action)
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)
				assert.Equal(t, map[string]interface{}{"index": map[string]interface{}{}}, action)

				doc := make(map[string]interface{})
				assert.NoError(t, d.Decode(&doc))
				assert.Equal(t, map[string]interface{}{"id": beatID}, doc["observer"])

				trace := doc["trace"].(map[string]interface{})
				traceID := trace["id"].(string)
				received = append(received, traceID)
				delete(trace, "id")
				assert.Empty(t, trace) // no other fields in "trace"

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
	const (
		indexName = "trace-ids"
		beatID    = "beat_id"
	)

	var requests []*http.Request
	responses := make(chan string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r.Body); err != nil {
			panic(err)
		}
		r.Body = ioutil.NopCloser(&buf)
		requests = append(requests, r)

		select {
		case <-r.Context().Done():
		case resp := <-responses:
			w.Write([]byte(resp))
		}
	}))
	defer srv.Close()

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{srv.URL},
	})
	require.NoError(t, err)

	sub, err := pubsub.New(pubsub.Config{
		Client:         client,
		Index:          indexName,
		BeatID:         beatID,
		FlushInterval:  time.Minute,
		SearchInterval: time.Millisecond,
	})
	require.NoError(t, err)

	ids := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	go g.Go(func() error {
		return sub.SubscribeSampledTraceIDs(ctx, ids)
	})
	defer g.Wait()
	defer cancel()

	responses <- `{
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 1,
	        "_primary_term": 1,
		"_source": {"trace": {"id": "trace_1"}, "observer": {"id": "another_beat_id"}}
	      },
	      {
	        "_seq_no": 2,
	        "_primary_term": 2,
		"_source": {"trace": {"id": "trace_2"}, "observer": {"id": "another_beat_id"}}
	      }
	    ]
	  }
	}`

	assert.Equal(t, "trace_1", expectValue(t, ids))
	assert.Equal(t, "trace_2", expectValue(t, ids))

	responses <- "nonsense" // bad response, subscriber continues

	// trace_2 is repeated, since we search for >= the last
	// _seq_no, in case there's a new _primary_term.
	responses <- `{
          "hits": {
	    "hits": [
	      {
	        "_seq_no": 2,
	        "_primary_term": 2,
		"_source": {"trace": {"id": "trace_2"}, "observer": {"id": "another_beat_id"}}
	      },
	      {
	        "_seq_no": 2,
	        "_primary_term": 3,
		"_source": {"trace": {"id": "trace_2b"}, "observer": {"id": "another_beat_id"}}
	      },
	      {
	        "_seq_no": 99,
	        "_primary_term": 3,
		"_source": {"trace": {"id": "trace_99"}, "observer": {"id": "another_beat_id"}}
	      }
	    ]
	  }
	}`

	assert.Equal(t, "trace_2b", expectValue(t, ids))
	assert.Equal(t, "trace_99", expectValue(t, ids))

	responses <- `{"hits":{"hits":[]}}` // no hits
	expectNone(t, ids)

	cancel() // stop subscriber
	srv.Close()

	var bodies []string
	for _, r := range requests {
		assert.Equal(t, fmt.Sprintf("/%s/_search", indexName), r.URL.Path)

		var buf bytes.Buffer
		io.Copy(&buf, r.Body)
		bodies = append(bodies, strings.TrimSpace(buf.String()))
	}

	assert.Equal(t, []string{
		`{"query":{"bool":{"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[-1],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}]}`,

		// Repeats because of the invalid response.
		`{"query":{"bool":{"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[1],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}]}`,
		`{"query":{"bool":{"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[1],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}]}`,

		// Repeats because of the zero hits response.
		`{"query":{"bool":{"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[98],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}]}`,
		`{"query":{"bool":{"must_not":{"term":{"observer.id":{"value":"beat_id"}}}}},"search_after":[98],"seq_no_primary_term":true,"size":1000,"sort":[{"_seq_no":"asc"}]}`,
	}, bodies)
}

func expectValue(t testing.TB, ch <-chan string) string {
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
	select {
	case <-time.After(500 * time.Millisecond):
	case v := <-ch:
		t.Errorf("unexpected send on channel: %q", v)
	}
}
