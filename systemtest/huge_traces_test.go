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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.elastic.co/apm"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestTransactionDroppedSpansStats(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		ServiceDestinations: &apmservertest.ServiceDestinationAggregationConfig{
			Interval: time.Second,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	tracer := srv.Tracer()
	tx := tracer.StartTransaction("huge-traces", "type")

	// These spans are dropped since their duration < `exit_span_min_duration`.
	for i := 0; i < 50; i++ {
		span := tx.StartSpanOptions("EXISTS", "db.redis", apm.SpanOptions{
			ExitSpan: true,
		})
		span.Duration = 100 * time.Microsecond
		span.Outcome = "success"
		span.End()
	}
	for i := 0; i < 4; i++ {
		span := tx.StartSpanOptions("_bulk", "db.elasticsearch", apm.SpanOptions{
			ExitSpan: true,
		})
		span.Duration = 900 * time.Microsecond
		span.Outcome = "success"
		span.End()
	}

	tx.Duration = 30 * time.Millisecond
	tx.Outcome = "success"
	tx.End()
	tracer.Flush(nil)

	metricsResult := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm*metric",
		estest.TermQuery{Field: "metricset.name", Value: "service_destination"},
	)
	systemtest.ApproveEvents(t, t.Name()+"Metrics", metricsResult.Hits.Hits, "@timestamp")

	txResult := systemtest.Elasticsearch.ExpectDocs(t, "apm*transaction",
		estest.TermQuery{Field: "transaction.id", Value: tx.TraceContext().Span.String()},
	)
	systemtest.ApproveEvents(t, t.Name()+"Transaction", txResult.Hits.Hits,
		"@timestamp", "timestamp", "trace.id", "transaction.id",
	)
}

func TestCompressedSpans(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Aggregation = &apmservertest.AggregationConfig{
		ServiceDestinations: &apmservertest.ServiceDestinationAggregationConfig{
			Interval: time.Second,
		},
	}
	err := srv.Start()
	require.NoError(t, err)

	tracer := srv.Tracer()
	tracer.SetSpanCompressionEnabled(true)
	tx := tracer.StartTransaction("compressed-traces", "type")

	startTs := time.Now()

	// These spans will be compressed. The redis spans will be compressed
	// using the "same_kind" stategy, while the Elasticsearch spans will
	// use the "exact_match" strategy.
	for i := 0; i < 50; i++ {
		cmd := "EXISTS"
		if i%2 == 0 {
			cmd = "GET"
		}
		span := tx.StartSpanOptions(cmd, "db.redis", apm.SpanOptions{
			ExitSpan: true, Start: startTs,
		})
		span.Duration = 1 * time.Millisecond
		startTs = startTs.Add(span.Duration)
		span.Outcome = "success"
		span.End()
	}
	for i := 0; i < 5; i++ {
		span := tx.StartSpanOptions("_bulk", "db.elasticsearch", apm.SpanOptions{
			ExitSpan: true, Start: startTs,
		})
		span.Duration = 4 * time.Millisecond
		startTs = startTs.Add(span.Duration)
		span.Outcome = "success"
		span.End()
	}

	tx.Duration = 80 * time.Millisecond
	tx.Outcome = "success"
	tx.End()
	tracer.Flush(nil)

	txIgnoreFields := []string{"@timestamp", "timestamp", "trace.id", "transaction.id"}
	spanResults := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm*span",
		estest.TermQuery{Field: "span.type", Value: "db"},
	)
	systemtest.ApproveEvents(t, t.Name(), spanResults.Hits.Hits,
		append(txIgnoreFields, "span.id", "parent")...,
	)
}
