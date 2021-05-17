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

package beater

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/beater/api"
)

// transactions from testdata/intake-v2/transactions.ndjson used to trigger tracing
var testTransactionIds = map[string]bool{
	"945254c567a5417e":     true,
	"4340a8e0df1906ecbfa9": true,
	"cdef4340a8e0df19":     true,
	"00xxxxFFaaaa1234":     true,
}

func TestServerTracingEnabled(t *testing.T) {
	events, teardown := setupTestServerInstrumentation(t, true)
	defer teardown()

	txEvents := transactionEvents(events)
	var selfTransactions []string
	for len(selfTransactions) < 2 {
		select {
		case e := <-txEvents:
			if testTransactionIds[eventTransactionId(e)] {
				continue
			}

			// Check that self-instrumentation goes through the
			// reporter wrapped by setupBeater.
			wrapped, _ := e.GetValue("labels.wrapped_reporter")
			assert.Equal(t, true, wrapped)

			selfTransactions = append(selfTransactions, eventTransactionName(e))
		case <-time.After(5 * time.Second):
			assert.FailNow(t, "timed out waiting for transaction")
		}
	}
	assert.Contains(t, selfTransactions, "POST "+api.IntakePath)
	assert.Contains(t, selfTransactions, "ProcessPending")

	// We expect no more events, i.e. no recursive self-tracing.
	for {
		select {
		case e := <-txEvents:
			assert.FailNowf(t, "unexpected event", "%v", e)
		case <-time.After(time.Second):
			return
		}
	}
}

func TestServerTracingDisabled(t *testing.T) {
	events, teardown := setupTestServerInstrumentation(t, false)
	defer teardown()

	txEvents := transactionEvents(events)
	for {
		select {
		case e := <-txEvents:
			assert.Contains(t, testTransactionIds, eventTransactionId(e))
		case <-time.After(time.Second):
			return
		}
	}
}

func eventTransactionId(event beat.Event) string {
	transaction := event.Fields["transaction"].(common.MapStr)
	return transaction["id"].(string)
}

func eventTransactionName(event beat.Event) string {
	transaction := event.Fields["transaction"].(common.MapStr)
	return transaction["name"].(string)
}

func transactionEvents(events <-chan beat.Event) <-chan beat.Event {
	out := make(chan beat.Event, 1)
	go func() {
		defer close(out)
		for event := range events {
			processor := event.Fields["processor"].(common.MapStr)
			if processor["event"] == "transaction" {
				out <- event
			}
		}
	}()
	return out
}

// setupTestServerInstrumentation sets up a beater with or without instrumentation enabled,
// and returns a channel to which events are published, and a function to be
// called to teardown the beater. The initial onboarding event is consumed
// and a transactions request is made before returning.
func setupTestServerInstrumentation(t *testing.T, enabled bool) (chan beat.Event, func()) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	os.Setenv("ELASTIC_APM_API_REQUEST_TIME", "100ms")
	defer os.Unsetenv("ELASTIC_APM_API_REQUEST_TIME")

	events := make(chan beat.Event, 10)

	cfg := common.MustNewConfigFrom(m{
		"instrumentation": m{"enabled": enabled},
		"host":            "localhost:0",
		"secret_token":    "foo",
	})
	beater, err := setupServer(t, cfg, nil, events)
	require.NoError(t, err)

	// onboarding event
	e := <-events
	assert.Equal(t, "onboarding", e.Fields["processor"].(common.MapStr)["name"])

	// Send a transaction request so we have something to trace.
	req := makeTransactionRequest(t, beater.baseURL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	req.Header.Add("Authorization", "Bearer foo")
	resp, err := beater.client.Do(req)
	assert.NoError(t, err)
	resp.Body.Close()

	return events, beater.Stop
}
