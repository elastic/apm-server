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
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestRUMErrorSourcemapping(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	sourcemap, err := ioutil.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, string(sourcemap), "apm-agent-js", "1.0.1",
		"http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map",
	)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
	)
}

func TestRUMSpanSourcemapping(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	sourcemap, err := ioutil.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, string(sourcemap), "apm-agent-js", "1.0.0",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-span", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
	)
}

func TestNoMatchingSourcemap(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	// upload sourcemap with a wrong service version
	sourcemap, err := ioutil.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, string(sourcemap), "apm-agent-js", "2.0",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-span", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
	)
}

func TestSourcemapCaching(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	sourcemap, err := ioutil.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	sourcemapID := systemtest.CreateSourceMap(t, string(sourcemap), "apm-agent-js", "1.0.1",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)

	// Index an error, applying source mapping and caching the source map in the process.
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, true)

	// Delete the source map and error, and try again.
	systemtest.DeleteSourceMap(t, sourcemapID)
	deleteIndex(t, "apm-*-error*")
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result = systemtest.Elasticsearch.ExpectMinDocs(t, 1, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, true)
}

func deleteIndex(t *testing.T, name string) {
	resp, err := systemtest.Elasticsearch.Indices.Delete([]string{name})
	require.NoError(t, err)
	resp.Body.Close()
	resp, err = systemtest.Elasticsearch.Indices.Flush()
	require.NoError(t, err)
	resp.Body.Close()
}

func assertSourcemapUpdated(t *testing.T, result estest.SearchResult, updated bool) {
	t.Helper()

	type StacktraceFrame struct {
		Sourcemap struct {
			Updated bool
		}
	}
	type Error struct {
		Exception []struct {
			Stacktrace []StacktraceFrame
		}
		Log struct {
			Stacktrace []StacktraceFrame
		}
	}

	for _, hit := range result.Hits.Hits {
		var source struct {
			Error Error
		}
		err := hit.UnmarshalSource(&source)
		require.NoError(t, err)

		for _, exception := range source.Error.Exception {
			for _, stacktrace := range exception.Stacktrace {
				assert.Equal(t, updated, stacktrace.Sourcemap.Updated)
			}
		}

		for _, stacktrace := range source.Error.Log.Stacktrace {
			assert.Equal(t, updated, stacktrace.Sourcemap.Updated)
		}
	}
}
