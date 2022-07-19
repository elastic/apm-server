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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	agentconfig "github.com/elastic/elastic-agent-libs/config"
)

func TestServerTracingEnabled(t *testing.T) {
	t.Setenv("ELASTIC_APM_API_REQUEST_TIME", "10ms")

	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			cfg := agentconfig.MustNewConfigFrom(m{
				"host":                    "localhost:0",
				"instrumentation.enabled": enabled,
			})

			docs := make(chan []byte, 10)
			beater, err := setupServer(t, cfg, nil, docs)
			require.NoError(t, err)

			// Make an HTTP request to the server, which should be traced
			// if instrumentation is enabled.
			resp, err := beater.client.Get(beater.baseURL + "/foo")
			assert.NoError(t, err)
			resp.Body.Close()

			if enabled {
				// There will be some internal trace events before the transaction
				// corresponding to "GET /foo" above, so consume documents until we
				// find the one we are interested in.
				for {
					var doc []byte
					select {
					case doc = <-docs:
					case <-time.After(10 * time.Second):
						t.Fatal("timed out waiting for event")
					}
					transactionName := gjson.GetBytes(doc, "transaction.name")
					if transactionName.Exists() && transactionName.String() == "GET unknown route" {
						break
					}
				}
			}

			// There should be no more "request" transactions: there may be ongoing
			// "flush" requests for the output. Consume documents for a little while
			// to ensure there are no more "request" transactions.
			var done bool
			var traced bool
			timeout := time.After(100 * time.Millisecond)
			for !done {
				select {
				case doc := <-docs:
					traced = true
					transactionType := gjson.GetBytes(doc, "transaction.type")
					assert.NotEqual(t, "request", transactionType.String())
				case <-timeout:
					done = true
				}
			}
			if !enabled {
				assert.False(t, traced)
			}
		})
	}
}
