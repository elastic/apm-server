// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pubsub_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub"
)

const (
	defaultElasticsearchHost = "localhost"
	defaultElasticsearchPort = "9200"
	defaultElasticsearchUser = "apm_server_user"
	defaultElasticsearchPass = "changeme"
)

func TestElasticsearchIntegration_PublishSampledTraceIDs(t *testing.T) {
	const (
		localBeatID = "local_beat_id"
		indexName   = "apm-testing-sampled-traces"
	)

	client := newElasticsearchClient(t)
	recreateIndex(t, client, indexName)

	var input []string
	for i := 0; i < 50; i++ {
		input = append(input, uuid.Must(uuid.NewV4()).String())
	}

	es, err := pubsub.New(pubsub.Config{
		Client:         client,
		Index:          indexName,
		BeatID:         localBeatID,
		FlushInterval:  100 * time.Millisecond,
		SearchInterval: time.Minute,
	})
	require.NoError(t, err)

	err = es.PublishSampledTraceIDs(context.Background(), input...)
	assert.NoError(t, err)

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Observer struct {
						ID string
					}
					Trace struct {
						ID string
					}
				} `json:"_source"`
			}
		}
	}

	for {
		size := len(input) + 1
		resp, err := esapi.SearchRequest{
			Index: []string{indexName},
			Size:  &size,
		}.Do(context.Background(), client)
		require.NoError(t, err)
		if resp.IsError() {
			resp.Body.Close()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)
		resp.Body.Close()
		if len(result.Hits.Hits) == len(input) {
			break
		}
	}

	output := make([]string, len(input))
	for i, hit := range result.Hits.Hits {
		assert.Equal(t, localBeatID, hit.Source.Observer.ID)
		output[i] = hit.Source.Trace.ID
	}
	assert.ElementsMatch(t, input, output)
}

func TestElasticsearchIntegration_SubscribeSampledTraceIDs(t *testing.T) {
	const (
		localBeatID  = "local_observer_id"
		remoteBeatID = "remote_observer_id"
		indexName    = "apm-testing-sampled-traces"
	)

	client := newElasticsearchClient(t)
	recreateIndex(t, client, indexName)

	es, err := pubsub.New(pubsub.Config{
		Client:         client,
		Index:          indexName,
		BeatID:         localBeatID,
		FlushInterval:  time.Minute,
		SearchInterval: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	var g errgroup.Group
	out := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g.Go(func() error {
		return es.SubscribeSampledTraceIDs(ctx, out)
	})
	assert.NoError(t, err)

	// Nothing should be sent until there's a document indexed.
	expectNone(t, out)

	type indexAction struct {
		Index struct{} `json:"index"`
	}
	type doc struct {
		Observer struct {
			ID string `json:"id"`
		} `json:"observer"`
		Trace struct {
			ID string `json:"id"`
		} `json:"trace"`
	}
	indexTraceID := func(observerID string, traceID ...string) {
		var body bytes.Buffer
		enc := json.NewEncoder(&body)
		for _, id := range traceID {
			var doc doc
			doc.Observer.ID = observerID
			doc.Trace.ID = id
			assert.NoError(t, enc.Encode(indexAction{}))
			assert.NoError(t, enc.Encode(&doc))
		}
		resp, err := esapi.BulkRequest{
			Index: indexName,
			Body:  &body,
		}.Do(context.Background(), client)
		require.NoError(t, err)
		assert.False(t, resp.IsError())
		resp.Body.Close()
	}

	// Index some local observations. These should not be reported
	// by the local subscriber.
	var input []string
	for i := 0; i < 500; i++ {
		input = append(input, uuid.Must(uuid.NewV4()).String())
	}
	indexTraceID(localBeatID, input...)
	expectNone(t, out)

	// Index some remote observations. Repeat twice, to ensure that the
	// subscriber does not report old trace IDs the second time around.
	for i := 0; i < 2; i++ {
		var input []string
		for i := 0; i < 500; i++ {
			input = append(input, uuid.Must(uuid.NewV4()).String())
		}
		indexTraceID(remoteBeatID, input...)

		output := make([]string, len(input))
		for i := range input {
			output[i] = expectValue(t, out)
		}
		assert.Equal(t, input, output)
	}
}

func recreateIndex(tb testing.TB, client elasticsearch.Client, indexName string) {
	body := strings.NewReader(`{
  "mappings": {
    "properties": {
      "event.ingested": {"type": "date"},
      "observer": {
        "properties": {
          "id": {"type": "keyword"}
        }
      },
      "trace": {
        "properties": {
          "id": {"type": "keyword"}
        }
      }
    }
  }
}`)

	resp, err := esapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}.Do(context.Background(), client)
	require.NoError(tb, err)
	resp.Body.Close()

	resp, err = esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  body,
	}.Do(context.Background(), client)
	require.NoError(tb, err)
	assert.False(tb, resp.IsError())
	resp.Body.Close()
}

func newElasticsearchClient(tb testing.TB) elasticsearch.Client {
	switch strings.ToLower(os.Getenv("INTEGRATION_TESTS")) {
	case "1", "true":
	default:
		tb.Skip("Skipping integration test, export INTEGRATION_TESTS=1 to run")
	}

	esHost := net.JoinHostPort(
		getenvDefault("ES_HOST", defaultElasticsearchHost),
		getenvDefault("ES_PORT", defaultElasticsearchPort),
	)
	client, err := elasticsearch.NewClient(&elasticsearch.Config{
		Hosts:    []string{esHost},
		Username: getenvDefault("ES_USER", defaultElasticsearchUser),
		Password: getenvDefault("ES_PASS", defaultElasticsearchPass),
	})
	require.NoError(tb, err)
	return client
}

func getenvDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		v = defaultValue
	}
	return v
}
