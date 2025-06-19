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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestIntake(t *testing.T) {
	for _, tc := range []struct {
		filename      string
		name          string
		dynamicFields []string
	}{
		{filename: "errors.ndjson", name: "Errors"},
		{filename: "errors_transaction_id.ndjson", name: "ErrorsTxID"},
		{filename: "events.ndjson", name: "Events"},
		{filename: "metricsets.ndjson", name: "Metricsets"},
		{filename: "spans.ndjson", name: "Spans"},
		{filename: "transactions.ndjson", name: "Transactions"},
		{filename: "transactions-huge_traces.ndjson", name: "TransactionsHugeTraces"},
		{filename: "unknown-span-type.ndjson", name: "UnknownSpanType"},
		{
			filename:      "minimal.ndjson",
			name:          "MinimalEvents",
			dynamicFields: []string{"@timestamp", "timestamp.us"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := apmservertest.NewServerTB(t)
			systemtest.CleanupElasticsearch(t)
			response := systemtest.SendBackendEventsPayload(t, srv.URL, "../testdata/intake-v2/"+tc.filename)
			result := estest.ExpectMinDocs(t, systemtest.Elasticsearch,
				response.Accepted, "traces-apm*,metrics-apm*,logs-apm*",
				// Exclude aggregated transaction/service_destination metrics.
				// Aggregations are flushed on 1m/10m/60m boundaries, so even
				// if the test is fast there's a possibility of aggregated
				// metrics being returned.
				espoll.BoolQuery{
					MustNot: []any{espoll.ExistsQuery{Field: "metricset.interval"}},
				},
			)
			tc.dynamicFields = append(tc.dynamicFields,
				"client.geo.city_name",
				"client.geo.location",
				"client.geo.region_iso_code",
				"client.geo.region_name",
				"error.grouping_key",
			)
			approvaltest.ApproveFields(t, t.Name(), result.Hits.Hits, tc.dynamicFields...)
		})
	}

}

func TestIntakeMalformed(t *testing.T) {
	// Setup a custom ingest pipeline to test a malformed data ingestion.
	r, err := systemtest.Elasticsearch.Ingest.PutPipeline(
		"traces-apm@custom",
		strings.NewReader(`{"processors":[{"set":{"field":"span.duration.us","value":"poison"}}]}`),
	)
	require.NoError(t, err)
	require.False(t, r.IsError())
	defer systemtest.Elasticsearch.Ingest.DeletePipeline("traces-apm@custom")
	// Test malformed intake data.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := apmservertest.NewServerTB(t)
	systemtest.CleanupElasticsearch(t)
	response := systemtest.SendBackendEventsPayload(t, srv.URL, "../testdata/intake-v2/spans.ndjson")
	_, err = systemtest.Elasticsearch.SearchIndexMinDocs(
		ctx,
		response.Accepted,
		"traces-apm*",
		nil,
		espoll.WithTimeout(10*time.Second),
	)
	require.Error(t, err, "No traces should be indexed due to traces-apm@custom pipeline")
}
