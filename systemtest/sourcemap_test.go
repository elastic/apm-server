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
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
	"github.com/elastic/apm-tools/pkg/approvaltest"
	"github.com/elastic/apm-tools/pkg/espoll"
)

func TestRUMErrorSourcemapping(t *testing.T) {
	sourcemap, err := os.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)

	test := func(t *testing.T, bundleFilepath string) {
		systemtest.CleanupElasticsearch(t)

		systemtest.CreateSourceMap(t, sourcemap, "apm-agent-js", "1.0.1", bundleFilepath)

		test := func(t *testing.T, serverURL string) {
			retry := func() {
				systemtest.SendRUMEventsPayload(t, serverURL, "../testdata/intake-v2/errors_rum.ndjson")
			}
			result := estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "logs-apm.error-*", retry, nil, true)
			approvaltest.ApproveEvents(
				t, t.Name(), result.Hits.Hits,
				// RUM timestamps are set by the server based on the time the payload is received.
				"@timestamp", "timestamp.us",
				// RUM events have the source IP and port recorded, which are dynamic in the tests
				"client.ip", "source.ip", "source.port",
			)
		}

		t.Run("standalone", func(t *testing.T) {
			srv := apmservertest.NewUnstartedServerTB(t)
			srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
			// disable kibana to make sure there is no fallback logic
			srv.Config.Kibana.Enabled = false
			err := srv.Start()
			require.NoError(t, err)
			test(t, srv.URL)
		})
	}

	t.Run("absolute_bundle_filepath", func(t *testing.T) {
		test(t, "http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map")
	})

	t.Run("relative_bundle_filepath", func(t *testing.T) {
		test(t, "/test/e2e/general-usecase/bundle.js.map")
	})
}

func TestRUMSpanSourcemapping(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	sourcemap, err := os.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, sourcemap, "apm-agent-js", "1.0.0",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)

	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	srv.Config.Kibana.Enabled = false
	err = srv.Start()
	require.NoError(t, err)

	retry := func() {
		systemtest.SendRUMEventsPayload(t, srv.URL, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	}
	result := estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "traces-apm*", retry, espoll.TermQuery{
		Field: "processor.event",
		Value: "span",
	}, true)

	approvaltest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
	)
}

func TestNoMatchingSourcemap(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	// upload sourcemap with a wrong service version
	sourcemap, err := os.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, sourcemap, "apm-agent-js", "2.0",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)

	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	srv.Config.Kibana.Enabled = false
	err = srv.Start()
	require.NoError(t, err)

	retry := func() {
		systemtest.SendRUMEventsPayload(t, srv.URL, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	}
	result := estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "traces-apm*", retry, espoll.TermQuery{
		Field: "processor.event",
		Value: "span",
	}, false)

	approvaltest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
		// RUM events have the source port recorded, and in the tests it will be dynamic
		"source.port",
	)
}

func TestSourcemapCaching(t *testing.T) {
	systemtest.CleanupElasticsearch(t)

	sourcemap, err := os.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)
	systemtest.CreateSourceMap(t, sourcemap, "apm-agent-js", "1.0.1",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
	)

	srv := apmservertest.NewUnstartedServerTB(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	srv.Config.Kibana.Enabled = false
	err = srv.Start()
	require.NoError(t, err)

	// Index an error, applying source mapping and caching the source map in the process.
	retry := func() {
		systemtest.SendRUMEventsPayload(t, srv.URL, "../testdata/intake-v2/errors_rum.ndjson")
	}
	estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "logs-apm.error-*", retry, nil, true)

	// Delete the source map and error, and try again.
	systemtest.CleanupElasticsearch(t)
	estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "logs-apm.error-*", retry, nil, true)
}

func TestSourcemapFetcher(t *testing.T) {
	sourcemap, err := os.ReadFile("../testdata/sourcemap/bundle.js.map")
	require.NoError(t, err)

	testCases := []struct {
		name          string
		disableKibana bool
		rumESConfig   *apmservertest.ElasticsearchOutputConfig
	}{
		{
			name:          "elasticsearch",
			disableKibana: true,
		}, {
			name: "kibana fallback with es unreachable",
			rumESConfig: &apmservertest.ElasticsearchOutputConfig{
				// Use an unreachable address
				Hosts: []string{"127.0.0.1:12345"},
			},
		}, {
			name: "kibana fallback with es credentials unauthorized",
			rumESConfig: &apmservertest.ElasticsearchOutputConfig{
				// Use the wrong credentials so that the ES fetcher
				// will fail and apm server will fall back to kiban
				APIKey: "example",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			systemtest.CleanupElasticsearch(t)

			systemtest.CreateSourceMap(t, sourcemap, "apm-agent-js", "1.0.1",
				"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
			)

			srv := apmservertest.NewUnstartedServerTB(t)
			srv.Config.Kibana.Enabled = !tc.disableKibana
			srv.Config.RUM = &apmservertest.RUMConfig{
				Enabled: true,
				Sourcemap: &apmservertest.RUMSourcemapConfig{
					Enabled:  true,
					ESConfig: tc.rumESConfig,
				},
			}
			err = srv.Start()
			require.NoError(t, err)

			// Index an error, applying source mapping and caching the source map in the process.
			retry := func() {
				systemtest.SendRUMEventsPayload(t, srv.URL, "../testdata/intake-v2/errors_rum.ndjson")
			}
			estest.ExpectSourcemapError(t, systemtest.Elasticsearch, "logs-apm.error-*", retry, nil, true)
		})
	}
}

func deleteIndex(t *testing.T, name string) {
	resp, err := systemtest.Elasticsearch.Indices.Delete([]string{name})
	require.NoError(t, err)
	resp.Body.Close()
	resp, err = systemtest.Elasticsearch.Indices.Flush()
	require.NoError(t, err)
	resp.Body.Close()
}
