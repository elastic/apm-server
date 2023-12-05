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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
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

func TestApprovedMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)
	eventsPayload, err := os.ReadFile("../testdata/intake-v2/metricsets.ndjson")
	require.NoError(t, err)

	req, _ := http.NewRequest("POST", srv.URL+"/intake/v2/events?verbose=true", bytes.NewReader(eventsPayload))
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	var ingestResult struct {
		Accepted int
	}
	err = json.NewDecoder(resp.Body).Decode(&ingestResult)
	assert.NoError(t, err)

	// Check the metrics documents are exactly as we expect.
	indices := []string{"metrics-apm*"}
	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, ingestResult.Accepted, strings.Join(indices, ","), espoll.BoolQuery{
		Filter: []interface{}{
			espoll.TermQuery{Field: "processor.event", Value: "metric"},
		},
		MustNot: []interface{}{
			// Ignore server-produced transaction metrics; we're only interested in the metrics sent by the agent.
			espoll.TermQuery{Field: "metricset.name", Value: "transaction"},
		},
	})
	approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits)

	// Check dynamic mapping of histograms.
	mappings := getFieldMappings(t, indices, []string{"latency_distribution"})
	assert.Equal(t, map[string]interface{}{
		"latency_distribution": map[string]interface{}{
			"full_name": "latency_distribution",
			"mapping": map[string]interface{}{
				"latency_distribution": map[string]interface{}{
					"type": "histogram",
				},
			},
		},
	}, mappings)
}

func TestBreakdownMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	tracer := srv.Tracer()
	tx := tracer.StartTransaction("tx_name", "tx_type")
	span := tx.StartSpan("span_name", "span_type", nil)
	span.Duration = 500 * time.Millisecond
	span.End()
	tx.Duration = time.Second
	tx.End()
	tracer.SendMetrics(nil)
	tracer.Flush(nil)

	result := estest.ExpectMinDocs(t, systemtest.Elasticsearch, 2, "metrics-apm.internal-*", espoll.BoolQuery{
		Filter: []interface{}{
			espoll.TermQuery{
				Field: "processor.event",
				Value: "metric",
			},
			espoll.TermQuery{
				Field: "transaction.type",
				Value: "tx_type",
			},
		},
	})

	docs := unmarshalMetricsetDocs(t, result.Hits.Hits)
	assert.ElementsMatch(t, []metricsetDoc{{
		Trasaction:    metricsetTransaction{Type: "tx_type"},
		Span:          metricsetSpan{Type: "span_type"},
		MetricsetName: "span_breakdown",
	}, {
		Trasaction:    metricsetTransaction{Type: "tx_type"},
		Span:          metricsetSpan{Type: "app"},
		MetricsetName: "span_breakdown",
	}}, docs)
}

func TestApplicationMetrics(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServerTB(t)

	tracer := srv.Tracer()
	tracer.RegisterMetricsGatherer(apm.GatherMetricsFunc(func(ctx context.Context, metrics *apm.Metrics) error {
		metrics.Add("a.b.c", nil, 123)
		metrics.Add("x.y.z", nil, 123.456)
		return nil
	}))
	tracer.SendMetrics(nil)
	tracer.Flush(nil)

	result := estest.ExpectDocs(t, systemtest.Elasticsearch, "metrics-apm.app.*", espoll.TermQuery{
		Field: "metricset.name",
		Value: "app",
	})

	// The Go agent sends all metrics with the same set of labels in one metricset.
	// This includes custom metrics, Go runtime metrics, system and process metrics.
	expectedFields := []string{
		"golang.goroutines",
		"system.memory.total",
		"a.b.c",
		"x.y.z",
	}
	for _, fieldName := range expectedFields {
		var found bool
		for _, hit := range result.Hits.Hits {
			if _, ok := hit.Fields[fieldName]; ok {
				found = true
				break
			}
		}
		assert.True(t, found, "field %q not found in 'app' metricset docs", fieldName)
	}

	// Check that the index mapping has been updated for the custom
	// metrics, with the expected dynamically mapped field types.
	mappings := getFieldMappings(t, []string{"metrics-apm.app.*"}, []string{"a.b.c", "x.y.z"})
	assert.Equal(t, map[string]interface{}{
		"a.b.c": map[string]interface{}{
			"full_name": "a.b.c",
			"mapping": map[string]interface{}{
				"c": map[string]interface{}{
					"type":  "double",
					"index": false,
				},
			},
		},
		"x.y.z": map[string]interface{}{
			"full_name": "x.y.z",
			"mapping": map[string]interface{}{
				"z": map[string]interface{}{
					"type":  "double",
					"index": false,
				},
			},
		},
	}, mappings)
}

func getFieldMappings(t testing.TB, index []string, fields []string) map[string]interface{} {
	var allMappings map[string]struct {
		Mappings map[string]interface{}
	}
	_, err := systemtest.Elasticsearch.Do(context.Background(), &esapi.IndicesGetFieldMappingRequest{
		Index:  index,
		Fields: fields,
	}, &allMappings)
	require.NoError(t, err)

	mappings := make(map[string]interface{})
	for _, index := range allMappings {
		for k, v := range index.Mappings {
			assert.NotContains(t, mappings, k, "field %q exists in multiple indices", k)
			mappings[k] = v
		}
	}
	return mappings
}

type metricsetTransaction struct {
	Type string `json:"type"`
}

type metricsetSpan struct {
	Type string `json:"type"`
}

type metricsetSample struct {
	Value float64 `json:"value"`
}

type metricsetDoc struct {
	Trasaction        metricsetTransaction `json:"transaction"`
	Span              metricsetSpan        `json:"span"`
	MetricsetName     string               `json:"metricset.name"`
	MetricsetInterval string               `json:"metricset.interval"`
	Labels            map[string]string    `json:"labels"`
}

func unmarshalMetricsetDocs(t testing.TB, hits []espoll.SearchHit) []metricsetDoc {
	var docs []metricsetDoc
	for _, hit := range hits {
		docs = append(docs, unmarshalMetricsetDoc(t, &hit))
	}
	return docs
}

func unmarshalMetricsetDoc(t testing.TB, hit *espoll.SearchHit) metricsetDoc {
	var doc metricsetDoc
	if err := hit.UnmarshalSource(&doc); err != nil {
		t.Fatal(err)
	}
	doc.MetricsetName = hit.Fields["metricset.name"][0].(string)
	if interval := hit.Fields["metricset.interval"]; len(interval) > 0 {
		doc.MetricsetInterval = interval[0].(string)
	}
	return doc
}
