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
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"go.elastic.co/apm"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/esutil"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestIngestPipelinePipeline(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	tracer := srv.Tracer()
	httpRequest := &http.Request{
		URL: &url.URL{},
		Header: http.Header{
			"User-Agent": []string{
				"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0",
			},
		},
	}
	tx := tracer.StartTransaction("name", "type")
	tx.Context.SetHTTPRequest(httpRequest)
	span1 := tx.StartSpan("name", "type", nil)
	// If a destination address is recorded, and it is a valid
	// IPv4 or IPv6 address, it will be copied to destination.ip.
	span1.Context.SetDestinationAddress("::1", 1234)
	span1.End()
	span2 := tx.StartSpan("name", "type", nil)
	span2.Context.SetDestinationAddress("testing.invalid", 1234)
	span2.End()
	tx.End()
	tracer.Flush(nil)

	getDoc := func(query estest.TermQuery) estest.SearchHit {
		result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", query)
		require.Len(t, result.Hits.Hits, 1)
		return result.Hits.Hits[0]
	}

	txDoc := getDoc(estest.TermQuery{Field: "processor.event", Value: "transaction"})
	assert.Equal(t, httpRequest.Header.Get("User-Agent"), gjson.GetBytes(txDoc.RawSource, "user_agent.original").String())
	assert.Equal(t, "Firefox", gjson.GetBytes(txDoc.RawSource, "user_agent.name").String())

	span1Doc := getDoc(estest.TermQuery{Field: "span.id", Value: span1.TraceContext().Span.String()})
	destinationIP := gjson.GetBytes(span1Doc.RawSource, "destination.ip")
	assert.True(t, destinationIP.Exists())
	assert.Equal(t, "::1", destinationIP.String())

	span2Doc := getDoc(estest.TermQuery{Field: "span.id", Value: span2.TraceContext().Span.String()})
	destinationIP = gjson.GetBytes(span2Doc.RawSource, "destination.ip")
	assert.False(t, destinationIP.Exists()) // destination.address is not an IP
}

func TestDataStreamMigrationIngestPipeline(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewServer(t)

	// Send a transaction, span, error, and metrics.
	tracer := srv.Tracer()
	tracer.RegisterMetricsGatherer(apm.GatherMetricsFunc(func(ctx context.Context, m *apm.Metrics) error {
		m.Add("custom_metric", nil, 123)
		return nil
	}))
	tx := tracer.StartTransaction("name", "type")
	span := tx.StartSpan("name", "type", nil)
	tracer.NewError(errors.New("boom")).Send()
	span.End()
	tx.End()
	tracer.Flush(nil)
	tracer.SendMetrics(nil)

	// We expect at least 6 events:
	// - onboarding
	// - transaction
	// - span
	// - error
	// - internal metricset
	// - app metricset
	for _, query := range []interface{}{
		estest.TermQuery{Field: "processor.event", Value: "onboarding"},
		estest.TermQuery{Field: "processor.event", Value: "transaction"},
		estest.TermQuery{Field: "processor.event", Value: "span"},
		estest.TermQuery{Field: "processor.event", Value: "error"},
		estest.TermQuery{Field: "metricset.name", Value: "span_breakdown"},
		estest.TermQuery{Field: "metricset.name", Value: "app"},
	} {
		systemtest.Elasticsearch.ExpectDocs(t, "apm-*", query)
	}

	refresh := true
	_, err := systemtest.Elasticsearch.Do(context.Background(), &esapi.ReindexRequest{
		Refresh: &refresh,
		Body: esutil.NewJSONReader(map[string]interface{}{
			"source": map[string]interface{}{
				"index": "apm-*",
			},
			"dest": map[string]interface{}{
				"index":    "apm-migration",
				"pipeline": "apm_data_stream_migration",
				"op_type":  "create",
			},
		}),
	}, nil)
	require.NoError(t, err)

	// There should only be an onboarding doc in "apm-migration".
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-migration", nil)
	require.Len(t, result.Hits.Hits, 1)
	assert.Equal(t, "onboarding", gjson.GetBytes(result.Hits.Hits[0].RawSource, "processor.event").String())

	systemtest.Elasticsearch.ExpectMinDocs(t, 2, "traces-apm-migrated", nil) // transaction, span
	systemtest.Elasticsearch.ExpectMinDocs(t, 1, "logs-apm.error-migrated", nil)
	systemtest.Elasticsearch.ExpectMinDocs(t, 1, "metrics-apm.internal-migrated", nil)
	systemtest.Elasticsearch.ExpectMinDocs(t, 1, "metrics-apm.app.systemtest-migrated", nil)
}
