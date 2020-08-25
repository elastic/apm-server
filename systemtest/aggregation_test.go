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
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestTransactionAggregation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
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
	//
	// Mimic a RUM transaction by using the "page-load" transaction type,
	// which causes user-agent to be parsed and included in the aggregation
	// and added to the document fields.
	tracer := srv.Tracer()
	const chromeUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.125 Safari/537.36"
	for _, transactionType := range []string{"backend", "page-load"} {
		tx := tracer.StartTransaction("name", transactionType)
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", chromeUserAgent)
		tx.Context.SetHTTPRequest(req)
		tx.Duration = time.Second
		tx.End()
	}
	tracer.Flush(nil)

	var result estest.SearchResult
	_, err = systemtest.Elasticsearch.Search("apm-*").WithQuery(estest.BoolQuery{
		Filter: []interface{}{
			estest.ExistsQuery{Field: "transaction.duration.histogram"},
		},
	}).Do(context.Background(), &result,
		estest.WithCondition(result.Hits.NonEmptyCondition()),
	)
	require.NoError(t, err)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
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
			Interval: time.Minute,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	// Send a transaction to the server to be aggregated.
	tracer := srv.Tracer()
	tx := tracer.StartTransaction("name", "type")
	tx.Duration = time.Second
	tx.End()
	tracer.Flush(nil)

	// Stop server to ensure metrics are flushed on shutdown.
	assert.NoError(t, srv.Close())

	var result estest.SearchResult
	_, err = systemtest.Elasticsearch.Search("apm-*").WithQuery(estest.BoolQuery{
		Filter: []interface{}{
			estest.ExistsQuery{Field: "transaction.duration.histogram"},
		},
	}).Do(context.Background(), &result,
		estest.WithCondition(result.Hits.NonEmptyCondition()),
	)
	require.NoError(t, err)
	systemtest.ApproveEvents(t, t.Name(), result.Hits.Hits)
}
