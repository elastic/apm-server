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
	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestTransactionAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Monitoring = &apmservertest.MonitoringConfig{
		Enabled:       true,
		MetricsPeriod: 100 * time.Millisecond,
		StatePeriod:   100 * time.Millisecond,
	}
	require.NoError(t, srv.Start())

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
	// Currently, the Go agent doesn't support setting FaaS fields.
	// TODO(marclop) Update the Go agent module to a newer version and test
	// when it does.
	faasPayload := `{"metadata": {"service": {"name": "1234_service-12a3","node": {"configured_name": "node-123"},"version": "5.1.3","environment": "staging","language": {"name": "ecmascript","version": "8"},"runtime": {"name": "node","version": "8.0.0"},"framework": {"name": "Express","version": "1.2.3"},"agent": {"name": "elastic-node","version": "3.14.0"}},"user": {"id": "123user", "username": "bar", "email": "bar@user.com"}, "labels": {"tag0": null, "tag1": "one", "tag2": 2}, "process": {"pid": 1234,"ppid": 6789,"title": "node","argv": ["node","server.js"]},"system": {"hostname": "prod1.example.com","architecture": "x64","platform": "darwin", "container": {"id": "container-id"}, "kubernetes": {"namespace": "namespace1", "pod": {"uid": "pod-uid", "name": "pod-name"}, "node": {"name": "node-name"}}},"cloud":{"account":{"id":"account_id","name":"account_name"},"availability_zone":"cloud_availability_zone","instance":{"id":"instance_id","name":"instance_name"},"machine":{"type":"machine_type"},"project":{"id":"project_id","name":"project_name"},"provider":"cloud_provider","region":"cloud_region","service":{"name":"lambda"}}}}
{"transaction": { "name": "faas", "type": "lambda", "result": "success", "id": "142e61450efb8574", "trace_id": "eb56529a1f461c5e7e2f66ecb075e983", "subtype": null, "action": null, "duration": 38.853, "timestamp": 1631736666365048, "sampled": true, "context": { "cloud": { "origin": { "account": { "id": "abc123" }, "provider": "aws", "region": "us-east-1", "service": { "name": "serviceName" } } }, "service": { "origin": { "id": "abc123", "name": "service-name", "version": "1.0" } }, "user": {}, "tags": {}, "custom": { } }, "sync": true, "span_count": { "started": 0 }, "outcome": "unknown", "faas": { "coldstart": false, "execution": "2e13b309-23e1-417f-8bf7-074fc96bc683", "trigger": { "request_id": "FuH2Cir_vHcEMUA=", "type": "http" } }, "sample_rate": 1 } }
`
	systemtest.SendBackendEventsLiteral(t, srv.URL, faasPayload)
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)

	// Make sure apm-server.aggregation.txmetrics metrics are published. Metric values are unit tested.
	doc := getBeatsMonitoringStats(t, srv, nil)
	assert.True(t, gjson.GetBytes(doc.RawSource, "beats_stats.metrics.apm-server.aggregation.txmetrics").Exists())

	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	// Wait for 9 documents to be indexed (3 transaction names * 3 integration intervals)
	result := systemtest.Elasticsearch.ExpectMinDocs(t, 9, "metrics-apm.transaction*",
		estest.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)

	// Make sure the _doc_count field is added such that aggregations return
	// the appropriate per-bucket doc_count values.
	result = estest.SearchResult{}
	_, err := systemtest.Elasticsearch.Do(context.Background(), &esapi.SearchRequest{
		Index: []string{"metrics-apm.transaction*"},
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
		{Key: "def", DocCount: 30}, // 10 * 3 buckets
		{Key: "abc", DocCount: 15}, // 5 * 3 buckets
		{Key: "faas", DocCount: 3}, // 1 * 3 buckets
	}, aggregationResult.Buckets)
}

func TestTransactionAggregationShutdown(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

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
	systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	result := systemtest.Elasticsearch.ExpectMinDocs(t, 3, "metrics-apm.transaction*",
		estest.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestServiceDestinationAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

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

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectMinDocs(t, 6, "traces-apm*", nil)
	require.NoError(t, srv.Close())

	result := systemtest.Elasticsearch.ExpectDocs(t, "metrics-apm.service_destination*",
		estest.ExistsQuery{Field: "span.destination.service.response_time.count"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestTransactionAggregationLabels(t *testing.T) {
	t.Setenv("ELASTIC_APM_GLOBAL_LABELS", "department_name=apm,organization=observability,company=elastic")
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	tracer := srv.Tracer()
	tx := tracer.StartTransaction("name", "type")
	// The label that's set below won't be set in the metricset labels since
	// the labels will be set on the event and not in the metadata object.
	tx.Context.SetLabel("mylabel", "myvalue")
	tx.Duration = time.Second
	tx.End()
	tracer.Flush(nil)
	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := systemtest.Elasticsearch.ExpectDocs(t, "metrics-apm.transaction*", estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{Field: "processor.event", Value: "metric"},
			estest.TermQuery{Field: "metricset.name", Value: "transaction"},
		},
	})

	var metricsets []metricsetDoc
	for _, interval := range []string{"1m", "10m", "60m"} {
		metricsets = append(metricsets, metricsetDoc{
			Trasaction:        metricsetTransaction{Type: "type"},
			MetricsetName:     "transaction",
			MetricsetInterval: interval,
			Labels: map[string]string{
				"department_name": "apm",
				"organization":    "observability",
				"company":         "elastic",
			},
		})
	}
	docs := unmarshalMetricsetDocs(t, result.Hits.Hits)
	assert.ElementsMatch(t, metricsets, docs)
}

func TestServiceMetricsAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	enabled := true
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		Service: &apmservertest.ServiceAggregationConfig{Enabled: &enabled},
	}
	err := srv.Start()
	require.NoError(t, err)

	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tracer := srv.Tracer()
	for _, txType := range []string{"type1", "type2"} {
		for _, txName := range []string{"name1", "name2"} { // shouldn't affect aggregation
			tx := tracer.StartTransactionOptions(txName, txType, apm.TransactionOptions{Start: timestamp})
			tx.Duration = time.Second
			tx.End()
		}
	}
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectMinDocs(t, 2, "traces-apm*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "metrics-apm.service*",
		estest.TermQuery{Field: "metricset.name", Value: "service"},
	)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}

func TestServiceMetricsAggregationLabels(t *testing.T) {
	t.Setenv("ELASTIC_APM_GLOBAL_LABELS", "department_name=apm,organization=observability,company=elastic")
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	enabled := true
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		Service: &apmservertest.ServiceAggregationConfig{Enabled: &enabled},
	}
	err := srv.Start()
	require.NoError(t, err)

	tracer := srv.Tracer()
	tx := tracer.StartTransaction("name", "type")
	// The label that's set below won't be set in the metricset labels since
	// the labels will be set on the event and not in the metadata object.
	tx.Context.SetLabel("mylabel", "myvalue")
	tx.Duration = time.Second
	tx.End()
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*",
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := systemtest.Elasticsearch.ExpectDocs(t, "metrics-apm.service*", estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{Field: "metricset.name", Value: "service"},
		},
	})

	docs := unmarshalMetricsetDocs(t, result.Hits.Hits)
	var metricsets []metricsetDoc
	for _, interval := range []string{"1m", "10m", "60m"} {
		metricsets = append(metricsets, metricsetDoc{
			Trasaction:        metricsetTransaction{Type: "type"},
			MetricsetInterval: interval,
			MetricsetName:     "service",
			Labels: map[string]string{
				"department_name": "apm",
				"organization":    "observability",
				"company":         "elastic",
			},
		})
	}
	assert.ElementsMatch(t, metricsets, docs)
}
