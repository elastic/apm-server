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
	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestTransactionAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
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
{"transaction": { "name": "faas", "type": "lambda", "result": "success", "id": "142e61450efb8575", "trace_id": "eb56529a1f461c5e7e2f66ecb075e984", "subtype": null, "action": null, "duration": 38.853, "timestamp": 1631736666365049, "sampled": true, "context": { "cloud": { "origin": { "account": { "id": "abc123" }, "provider": "aws", "region": "us-east-1", "service": { "name": "serviceName" } } }, "service": { "origin": { "id": "abc123", "name": "service-name", "version": "1.0" } }, "user": {}, "tags": {}, "custom": { } }, "sync": true, "span_count": { "started": 0 }, "outcome": "unknown", "faas": { "coldstart": false, "execution": "2e13b309-23e1-417f-8bf7-074fc96bc684", "trigger": { "request_id": "FuH2Cir_vHcEMUB=", "type": "http" } }, "sample_rate": 1 } }
`
	systemtest.SendBackendEventsLiteral(t, srv.URL, faasPayload)
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)

	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	// Wait for 9 documents to be indexed (3 transaction names * 3 integration intervals)
	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 9, "metrics-apm.transaction*",
		espoll.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)

	// Make sure the _doc_count field is added such that aggregations return
	// the appropriate per-bucket doc_count values.
	result = espoll.SearchResult{}
	_, err := systemtest.Elasticsearch.Do(context.Background(), &esapi.SearchRequest{
		Index:           []string{"metrics-apm.transaction*"},
		ExpandWildcards: "open,hidden",
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
		{Key: "faas", DocCount: 6}, // 2 * 3 buckets
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
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 3, "metrics-apm.transaction*",
		espoll.ExistsQuery{Field: "transaction.duration.histogram"},
	)
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)
}

func TestServiceDestinationAggregation(t *testing.T) {
	t.Setenv("ELASTIC_APM_GLOBAL_LABELS", "department_name=apm,organization=observability,company=elastic")
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	// Send spans to the server to be aggregated.
	tracer := srv.Tracer()
	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tx := tracer.StartTransaction("name", "type")
	// The label that's set below won't be set in the metricset labels since
	// the labels will be set on the event and not in the metadata object.
	tx.Context.SetLabel("mylabel", "myvalue")
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
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, 6, "traces-apm*", nil)
	require.NoError(t, srv.Close())

	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.service_destination*",
		espoll.ExistsQuery{Field: "span.destination.service.response_time.count"},
	)
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)

	// _doc_count is not returned in fields, it is only visible in _source and
	// in the results of aggregations.
	//
	// TODO(axw) we should use an aggregation, and check the resturned doc_counts.
	for _, hit := range result.Hits.Hits {
		docCount := hit.Source["_doc_count"].(float64)
		assert.Equal(t, 5.0, docCount)
	}
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
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.transaction*",
		espoll.BoolQuery{
			Filter: []interface{}{
				espoll.TermQuery{Field: "processor.event", Value: "metric"},
				espoll.TermQuery{Field: "metricset.name", Value: "transaction"},
			},
		},
	)

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

func TestServiceTransactionMetricsAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

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
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, 2, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 2, "metrics-apm.service_transaction*",
		espoll.TermQuery{Field: "metricset.name", Value: "service_transaction"},
	)
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)

	// _doc_count is not returned in fields, it is only visible in _source and
	// in the results of aggregations.
	//
	// TODO(axw) we should use an aggregation, and check the resturned doc_counts.
	for _, hit := range result.Hits.Hits {
		docCount := hit.Source["_doc_count"].(float64)
		assert.Equal(t, 2.0, docCount)
	}
}

func TestServiceTransactionMetricsAggregationLabels(t *testing.T) {
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
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.service_transaction*",
		espoll.BoolQuery{
			Filter: []interface{}{
				espoll.TermQuery{Field: "metricset.name", Value: "service_transaction"},
			},
		},
	)

	docs := unmarshalMetricsetDocs(t, result.Hits.Hits)
	var metricsets []metricsetDoc
	for _, interval := range []string{"1m", "10m", "60m"} {
		metricsets = append(metricsets, metricsetDoc{
			Trasaction:        metricsetTransaction{Type: "type"},
			MetricsetInterval: interval,
			MetricsetName:     "service_transaction",
			Labels: map[string]string{
				"department_name": "apm",
				"organization":    "observability",
				"company":         "elastic",
			},
		})
	}
	assert.ElementsMatch(t, metricsets, docs)
}

// TestServiceTransactionMetricsAggregationLabelsRUM checks that RUM labels are ignored for aggregation
func TestServiceTransactionMetricsAggregationLabelsRUM(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.RUM = &apmservertest.RUMConfig{
		Enabled: true,
	}
	err := srv.Start()
	require.NoError(t, err)

	rumPayloadWithLabels := `{"metadata":{"service":{"name":"rum-js-test","agent":{"name":"rum-js","version":"5.5.0"}},"labels": {"tag0": null, "tag1": "one", "tag2": 2}}}
{"transaction":{"trace_id":"611f4fa950f04631aaaaaaaaaaaaaaaa","id":"611f4fa950f04631","type":"page-load","duration":643,"span_count":{"started":0}}}
`
	systemtest.SendRUMEventsLiteral(t, srv.URL, rumPayloadWithLabels)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm.rum*", espoll.BoolQuery{
		Filter: []interface{}{
			espoll.TermQuery{Field: "processor.event", Value: "transaction"},
			espoll.TermQuery{Field: "labels.tag1", Value: "one"},
			espoll.TermQuery{Field: "numeric_labels.tag2", Value: 2},
		},
	})
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.service_transaction*", espoll.BoolQuery{
		Filter: []interface{}{
			espoll.TermQuery{Field: "metricset.name", Value: "service_transaction"},
		},
	})

	docs := unmarshalMetricsetDocs(t, result.Hits.Hits)
	var metricsets []metricsetDoc
	for _, interval := range []string{"1m", "10m", "60m"} {
		metricsets = append(metricsets, metricsetDoc{
			Trasaction:        metricsetTransaction{Type: "page-load"},
			MetricsetInterval: interval,
			MetricsetName:     "service_transaction",
		})
	}
	assert.ElementsMatch(t, metricsets, docs)
}

func TestServiceSummaryMetricsAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tracer := srv.Tracer()
	for i := 0; i < 2; i++ {
		tx := tracer.StartTransactionOptions("name", "type", apm.TransactionOptions{Start: timestamp})
		tx.Duration = time.Second
		tx.End()
	}
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, 2, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.service_summary*",
		espoll.TermQuery{Field: "metricset.name", Value: "service_summary"},
	)
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)
}

func TestServiceSummaryMetricsAggregationOverflow(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		MaxServices: 2,
	}
	require.NoError(t, srv.Start())

	sendTransaction := func(env string) {
		t.Setenv("ELASTIC_APM_ENVIRONMENT", env)
		timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
		tracer := srv.Tracer()
		tx := tracer.StartTransactionOptions("name", "type", apm.TransactionOptions{Start: timestamp})
		tx.Duration = time.Second
		tx.End()
		tracer.Flush(nil)
	}
	sendTransaction("dev")
	sendTransaction("staging")
	sendTransaction("prod")
	sendTransaction("test")

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, 4, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())
	estest.ExpectMinDocs(t, systemtest.Elasticsearch, 3, "metrics-apm.service_summary*",
		espoll.BoolQuery{
			Must: []interface{}{
				espoll.TermQuery{Field: "metricset.name", Value: "service_summary"},
				espoll.TermQuery{Field: "service.name", Value: "_other"},
			},
		},
	)
	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 9, "metrics-apm.service_summary*",
		espoll.TermQuery{Field: "metricset.name", Value: "service_summary"},
	)
	// Ignore timestamp because overflow bucket uses time.Now()
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits, "@timestamp")
}

func TestNonDefaultRollupIntervalHiddenDataStream(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	timestamp, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.999999999Z") // should be truncated to 1s
	tracer := srv.Tracer()
	tx := tracer.StartTransactionOptions("name", "type", apm.TransactionOptions{Start: timestamp})
	span := tx.StartSpanOptions("name", "type", apm.SpanOptions{Start: timestamp})
	span.Context.SetDestinationService(apm.DestinationServiceSpanContext{
		Name:     "name",
		Resource: "resource",
	})
	span.Duration = time.Second
	span.End()
	tx.End()
	tracer.Flush(nil)

	// Wait for the transaction to be indexed, indicating that Elasticsearch
	// indices have been setup and we should not risk triggering the shutdown
	// timeout while waiting for the aggregated metrics to be indexed.
	estest.ExpectDocs(t, systemtest.Elasticsearch, "traces-apm*",
		espoll.TermQuery{Field: "processor.event", Value: "transaction"},
	)
	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	indexPatterns := []string{
		"metrics-apm.transaction*",
		"metrics-apm.service_transaction*",
		"metrics-apm.service_summary*",
		"metrics-apm.service_destination*",
	}

	type Response struct {
		DataStreams []struct {
			Name   string `json:"name"`
			Hidden bool   `json:"hidden"`
		} `json:"data_streams"`
	}

	for _, pattern := range indexPatterns {
		result := Response{}
		req := esapi.IndicesGetDataStreamRequest{
			Name:            []string{pattern},
			ExpandWildcards: "open,hidden",
		}
		_, err := systemtest.Elasticsearch.Do(context.Background(), req, &result)
		assert.NoError(t, err)
		// 3 rollup intervals
		assert.Lenf(t, result.DataStreams, 3,
			"data stream pattern %s: expected to match %d data streams; actual: %d", pattern, 3, len(result.DataStreams))
		for _, ds := range result.DataStreams {
			// All non-default rollup interval data streams should be hidden
			expected := !strings.HasSuffix(ds.Name, ".1m-default")
			assert.Equalf(t, expected, ds.Hidden,
				"data stream name %s: .hidden expected: %v; actual: %v", ds.Name, expected, ds.Hidden)
		}
	}
}
