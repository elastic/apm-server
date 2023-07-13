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
	"sync"
	"testing"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestIntakeAsync(t *testing.T) {
	t.Run("semaphore_size_5", func(t *testing.T) {
		systemtest.CleanupElasticsearch(t)
		// limit the maximum concurrent decoders to 5. This will allow to test
		// for the successful case (no concurrency and unsuccessful case).
		srv := apmservertest.NewServerTB(t, "-E", "apm-server.max_concurrent_decoders=5")

		systemtest.SendBackendEventsAsyncPayload(t, srv.URL, `../testdata/intake-v2/errors.ndjson`)
		// Ensure the 5 errors are ingested.
		estest.ExpectMinDocs(t, systemtest.Elasticsearch, 5, "logs-apm.error-*", nil)

		// Send a request with a lot of events (~1920) and expect errors to be
		// returned, since the semaphore size is 5 (5 concurrent batches).
		systemtest.SendBackendEventsAsyncPayloadError(t, srv.URL, `../testdata/intake-v2/heavy.ndjson`)

		// Create 4 requests to be run concurrently and ensure that they return with
		// a 503
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				systemtest.SendBackendEventsAsyncPayloadError(t, srv.URL, `../testdata/intake-v2/heavy.ndjson`)
			}()
		}
		wg.Wait()
	})
	t.Run("semaphore_size_default", func(t *testing.T) {
		systemtest.CleanupElasticsearch(t)
		srv := apmservertest.NewServerTB(t)

		systemtest.SendBackendEventsAsyncPayload(t, srv.URL, `../testdata/intake-v2/errors.ndjson`)
		// Ensure the 5 errors are ingested.
		estest.ExpectMinDocs(t, systemtest.Elasticsearch, 5, "logs-apm.error-*", nil)

		// Send a request with a lot of events (~1920) and expect it to be processed
		// without any errors.
		systemtest.SendBackendEventsPayload(t, srv.URL, `../testdata/intake-v2/heavy.ndjson`)

		// Create 10 requests to be run concurrently and ensure that they succeed.
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// ratelimit.ndjson contains 20 events, which means that with
				// the default batch size of 10 it'll acquire the lock twice.
				systemtest.SendBackendEventsAsyncPayload(t, srv.URL, `../testdata/intake-v2/ratelimit.ndjson`)
			}()
		}
		wg.Wait()
	})
}
