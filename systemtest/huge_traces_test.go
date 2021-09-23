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

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestTransactionDroppedSpansStats(t *testing.T) {
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

	// TODO (marclop): use Go agent when the feature is available
	// (https://github.com/elastic/apm-agent-go/issues/1113).
	systemtest.SendBackendEventsPayload(t, srv, "../testdata/intake-v2/transactions-huge_traces.ndjson")

	metricsResult := systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm*metric",
		estest.TermQuery{Field: "metricset.name", Value: "service_destination"},
	)
	systemtest.ApproveEvents(t, t.Name()+"Metrics", metricsResult.Hits.Hits, "@timestamp")

	txResult := systemtest.Elasticsearch.ExpectDocs(t, "apm*transaction",
		estest.TermQuery{Field: "transaction.id", Value: "ddf109a4c4aa5f2b6e984548ca57774c"},
	)
	systemtest.ApproveEvents(t, t.Name()+"Transaction", txResult.Hits.Hits, "@timestamp")
}
