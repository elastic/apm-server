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

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
)

func TestIntake(t *testing.T) {
	srv := apmservertest.NewServerTB(t)
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
			systemtest.CleanupElasticsearch(t)
			response := systemtest.SendBackendEventsPayload(t, srv.URL, "../testdata/intake-v2/"+tc.filename)
			result := estest.ExpectMinDocs(t, systemtest.Elasticsearch,
				response.Accepted, "traces-apm*,metrics-apm*,logs-apm*", nil,
			)
			approvaltest.ApproveEvents(t, t.Name(), result.Hits.Hits, tc.dynamicFields...)
		})
	}

}
