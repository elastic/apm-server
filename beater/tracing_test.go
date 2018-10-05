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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	publishertesting "github.com/elastic/beats/libbeat/publisher/testing"
)

func TestServerTracingEnabled(t *testing.T) {
	events, teardown := setupTestServerInstrumentation(t, true)
	defer teardown()

	txEvents := transactionEvents(events)
	var selfTransactions []string
	for len(selfTransactions) < 2 {
		select {
		case e := <-txEvents:
			name := eventTransactionName(e)
			if name == "GET /api/types" {
				continue
			}
			selfTransactions = append(selfTransactions, name)
		case <-time.After(5 * time.Second):
			assert.FailNow(t, "timed out waiting for transaction")
		}
	}
	assert.Contains(t, selfTransactions, "POST "+BackendTransactionsURL)
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

func TestServerTracingExternal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	os.Setenv("ELASTIC_APM_FLUSH_INTERVAL", "100ms")
	defer os.Unsetenv("ELASTIC_APM_FLUSH_INTERVAL")

	// start up a fake remote apm-server
	requests := make(chan *http.Request)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		requests <- r
	}))
	defer remote.Close()

	// start a test apm-server
	ucfg, err := common.NewConfigFrom(m{"instrumentation": m{
		"enabled": true,
		"hosts":   "http://" + remote.Listener.Addr().String()}})
	apm, teardown, err := setupServer(t, ucfg, nil)
	require.NoError(t, err)
	defer teardown()

	// make a transaction request
	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))

	// ensure the transaction is reported to the remote apm-server
	select {
	case r := <-requests:
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/transactions", r.RequestURI)
	case <-time.After(time.Second):
		assert.FailNow(t, "timed out waiting for transaction to")
	}
}

func TestServerTracingDisabled(t *testing.T) {
	events, teardown := setupTestServerInstrumentation(t, false)
	defer teardown()

	txEvents := transactionEvents(events)
	for {
		select {
		case e := <-txEvents:
			assert.Equal(t, "GET /api/types", eventTransactionName(e))
		case <-time.After(time.Second):
			return
		}
	}
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

	os.Setenv("ELASTIC_APM_FLUSH_INTERVAL", "100ms")
	defer os.Unsetenv("ELASTIC_APM_FLUSH_INTERVAL")

	events := make(chan beat.Event, 10)
	pubClient := publishertesting.NewChanClientWith(events)
	pub := publishertesting.PublisherWithClient(pubClient)

	cfg, err := common.NewConfigFrom(m{
		"instrumentation": m{"enabled": enabled},
		"host":            "localhost:0",
	})
	assert.NoError(t, err)
	beater, teardown, err := setupBeater(t, pub, cfg, nil)
	require.NoError(t, err)

	// onboarding event
	e := <-events
	assert.Contains(t, e.Fields, "listening")

	// Send a transaction request so we have something to trace.
	baseUrl, client := beater.client(false)
	req := makeTransactionRequest(t, baseUrl)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	assert.NoError(t, err)
	resp.Body.Close()

	return events, teardown
}
