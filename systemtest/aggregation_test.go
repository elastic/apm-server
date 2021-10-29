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

package systemtest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.elastic.co/apm"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestTransactionAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Monitoring = &apmservertest.MonitoringConfig{
		Enabled:       true,
		MetricsPeriod: 100 * time.Millisecond,
		StatePeriod:   100 * time.Millisecond,
	}
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		Transactions: &apmservertest.TransactionAggregationConfig{
			Enabled:  true,
			Interval: time.Second,
		},
	}
	srv.Config.Sampling = &apmservertest.SamplingConfig{
		// Drop unsampled transaction events, to show
		// that we aggregate before they are dropped.
		KeepUnsampled: false,
	}
	err := srv.Start()
	require.NoError(t, err)

	// Send some transactions to the server to be aggregated.
	tracer := srv.Tracer()
	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	for i, name := range []string{"abc", "def"} {
		for j := 0; j < (i+1)*5; j++ {
			tx := tracer.StartTransactionOptions(name, "backend", apm.TransactionOptions{Start: timestamp})
			req, _ := http.NewRequest("GET", "/", nil)
			tx.Context.SetHTTPRequest(req)
			tx.Duration = time.Second
			tx.End()
		}
	}
	tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm-*",
		estest.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)

	// Make sure apm-server.aggregation.txmetrics metrics are published. Metric values are unit tested.
	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.aggregation.txmetrics").Exists())

	// Make sure the _doc_count field is added such that aggregations return
	// the appropriate per-bucket doc_count values.
	result = estest.SearchResult{}
	_, err = systemtest.Elasticsearch.Do(context.Background(), &esapi.SearchRequest{
		Index: []string{"apm-*"},
		Body: strings.NewReader(`{
  "size": 0,
  "query": {"exists":{"field":"transaction.duration.histogram"}},
  "aggs": {
    "transaction_names": {
      "terms": {"field": "transaction.name"}
    }
  }
}
`),
	}, &result)
	require.NoError(t, err)
	require.Contains(t, result.Aggregations, "transaction_names")

	type aggregationBucket struct {
		Key      string `json:"key"`
		DocCount int    `json:"doc_count"`
	}
	var aggregationResult struct {
		Buckets []aggregationBucket `json:"buckets"`
	}
	err = json.Unmarshal(result.Aggregations["transaction_names"], &aggregationResult)
	require.NoError(t, err)
	assert.Equal(t, []aggregationBucket{
		{Key: "def", DocCount: 10},
		{Key: "abc", DocCount: 5},
	}, aggregationResult.Buckets)
}

func TestTransactionAggregationShutdown(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		Transactions: &apmservertest.TransactionAggregationConfig{
			Enabled: true,
			// Set aggregation_interval to something that would cause
			// a timeout if we were to wait that long. The server
			// should flush metrics on shutdown without waiting for
			// the configured interval.
			Interval: 30 * time.Minute,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	// Send a transaction to the server to be aggregated.
	tracer := srv.Tracer()
	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tx := tracer.StartTransactionOptions("name", "type", apm.TransactionOptions{Start: timestamp})
	tx.Duration = time.Second
	tx.End()
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)

	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*",
		estest.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestServiceDestinationAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		ServiceDestinations: &apmservertest.ServiceDestinationAggregationConfig{
			Enabled:  true,
			Interval: time.Second,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	// Send spans to the server to be aggregated.
	tracer := srv.Tracer()
	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tx := tracer.StartTransaction("name", "type")
	for i := 0; i < 5; i++ {
		span := tx.StartSpanOptions("name", "type", apm.SpanOptions{Start: timestamp})
		span.Context.SetDestinationService(apm.DestinationServiceSpanContext{
			Name:     "name",
			Resource: "resource",
		})
		span.Duration = time.Second
		span.End()
	}
	tx.End()
	tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*",
		estest.ExistsQuery{Field: "span.destination.service.response_time.count"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}
