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
	"bytes"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/../e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.1",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
	)
}

func TestRUMSpanSourcemapping(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.0",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-span", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
	)
}

func TestDuplicateSourcemapWarning(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.0",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.0",
	)
	systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm-*-sourcemap", nil)

	require.NoError(t, srv.Close())
	var messages []string
	for _, entry := range srv.Logs.All() {
		messages = append(messages, entry.Message)
	}
	assert.Contains(t, messages,
		`Overriding sourcemap for service apm-agent-js version 1.0.0 and `+
			`file http://localhost:8000/test/e2e/general-usecase/bundle.js.map`,
	)
}

func TestNoMatchingSourcemap(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	// upload sourcemap with a wrong service version
	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"2.0",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/transactions_spans_rum_2.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-span", nil)

	systemtest.ApproveEvents(
		t, t.Name(), result.Hits.Hits,
		// RUM timestamps are set by the server based on the time the payload is received.
		"@timestamp", "timestamp.us",
	)
}

func TestFetchLatestSourcemap(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	// upload sourcemap file that finds no matchings
	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"2.0",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, false)
	deleteIndex(t, "apm-*-error*")

	// upload second sourcemap file with same key,
	// that actually leads to proper matchings
	// this also tests that the cache gets invalidated,
	// as otherwise the former sourcemap would be taken from the cache.
	// upload sourcemap file that finds no matchings
	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.1",
	)
	systemtest.Elasticsearch.ExpectMinDocs(t, 2, "apm-*-sourcemap", nil)

	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result = systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, true)
}

func TestSourcemapCaching(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	uploadSourcemap(t, srv, "../testdata/sourcemap/bundle.js.map",
		"http://localhost:8000/test/e2e/general-usecase/bundle.js.map",
		"apm-agent-js",
		"1.0.1",
	)
	systemtest.Elasticsearch.ExpectDocs(t, "apm-*-sourcemap", nil)

	// Index an error, applying source mapping and caching the source map in the process.
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result := systemtest.Elasticsearch.ExpectDocs(t, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, true)

	// Delete the source map and error, and try again.
	deleteIndex(t, "apm-*-sourcemap*")
	deleteIndex(t, "apm-*-error*")
	systemtest.SendRUMEventsPayload(t, srv, "../testdata/intake-v2/errors_rum.ndjson")
	result = systemtest.Elasticsearch.ExpectMinDocs(t, 1, "apm-*-error", nil)
	assertSourcemapUpdated(t, result, true)
}

func uploadSourcemap(t *testing.T, srv *apmservertest.Server, sourcemapFile, bundleFilepath, serviceName, serviceVersion string) {
	t.Helper()

	req := newUploadSourcemapRequest(t, srv, sourcemapFile, bundleFilepath, serviceName, serviceVersion)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode, string(respBody))
}

func newUploadSourcemapRequest(t *testing.T, srv *apmservertest.Server, sourcemapFile, bundleFilepath, serviceName, serviceVersion string) *http.Request {
	t.Helper()

	var data bytes.Buffer
	mw := multipart.NewWriter(&data)
	require.NoError(t, mw.WriteField("service_name", serviceName))
	require.NoError(t, mw.WriteField("service_version", serviceVersion))
	require.NoError(t, mw.WriteField("bundle_filepath", bundleFilepath))

	f, err := os.Open(sourcemapFile)
	require.NoError(t, err)
	defer f.Close()
	sourcemapFileWriter, err := mw.CreateFormFile("sourcemap", filepath.Base(sourcemapFile))
	require.NoError(t, err)
	_, err = io.Copy(sourcemapFileWriter, f)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, _ := http.NewRequest("POST", srv.URL+"/assets/v1/sourcemaps", &data)
	req.Header.Add("Content-Type", mw.FormDataContentType())
	return req
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
