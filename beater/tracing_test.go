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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestServerTracingEnabled(t *testing.T) {
	os.Setenv("ELASTIC_APM_API_REQUEST_TIME", "10ms")
	defer os.Unsetenv("ELASTIC_APM_API_REQUEST_TIME")

	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			cfg := common.MustNewConfigFrom(m{
				"host":                    "localhost:0",
				"instrumentation.enabled": enabled,
			})
			events := make(chan beat.Event, 10)
			beater, err := setupServer(t, cfg, nil, events)
			require.NoError(t, err)

			// Make an HTTP request to the server, which should be traced
			// if instrumentation is enabled.
			resp, err := beater.client.Get(beater.baseURL + "/foo")
			assert.NoError(t, err)
			resp.Body.Close()

			if enabled {
				select {
				case <-events:
				case <-time.After(10 * time.Second):
					t.Fatal("timed out waiting for event")
				}
			}

			// We expect no more events, i.e. no recursive self-tracing.
			select {
			case e := <-events:
				t.Errorf("unexpected event: %v", e)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}
