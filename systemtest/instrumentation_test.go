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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestAPMServerInstrumentation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Instrumentation = &apmservertest.InstrumentationConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	// Send a transaction to the server, causing the server to
	// trace the request from the agent.
	tracer := srv.Tracer()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*", estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{
				Field: "processor.event",
				Value: "transaction",
			},
			estest.TermQuery{
				Field: "service.name",
				Value: "apm-server",
			},
			estest.TermQuery{
				Field: "transaction.type",
				Value: "request",
			},
		},
	})

	var transactionDoc struct {
		Trace       struct{ ID string }
		Transaction struct{ ID string }
	}
	err = json.Unmarshal([]byte(result.Hits.Hits[0].RawSource), &transactionDoc)
	require.NoError(t, err)
	require.NotZero(t, transactionDoc.Trace.ID)
	require.NotZero(t, transactionDoc.Transaction.ID)

	// There should be a corresponding log record with matching
	// trace.id and transaction.id, which enables trace/log correlation.
	logs := srv.Logs.Iterator()
	defer logs.Close()
	for entry := range logs.C() {
		traceID, ok := entry.Fields["trace.id"]
		if !ok {
			continue
		}
		assert.Equal(t, transactionDoc.Trace.ID, traceID)
		assert.Equal(t, transactionDoc.Transaction.ID, entry.Fields["transaction.id"])
		return
	}
	t.Fatal("failed to identify log message with matching trace IDs")
}
