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
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestSourcemapIndexPattern(t *testing.T) {
	test := func(t *testing.T, indexPattern, expected string) {
		var requestPaths []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ignore duplicates: we might have two requests since the first one
			// will be from the init goroutine
			for _, v := range requestPaths {
				if v == r.URL.Path {
					return
				}
			}
			requestPaths = append(requestPaths, r.URL.Path)
		}))
		defer srv.Close()

		cfg := config.DefaultConfig()
		cfg.RumConfig.Enabled = true
		cfg.RumConfig.SourceMapping.ESConfig.Hosts = []string{srv.URL}
		if indexPattern != "" {
			cfg.RumConfig.SourceMapping.IndexPattern = indexPattern
		}

		fetcher, cancel, err := newSourcemapFetcher(
			cfg.RumConfig.SourceMapping,
			nil, elasticsearch.NewClient,
		)
		require.NoError(t, err)
		defer cancel()
		fetcher.Fetch(context.Background(), "name", "version", "path")
		require.Len(t, requestPaths, 1)

		path := requestPaths[0]
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/_search")
		assert.Equal(t, expected, path)
	}
	t.Run("default-pattern", func(t *testing.T) {
		test(t, "", ".apm-source-map")
	})
	t.Run("with-observer-version", func(t *testing.T) {
		test(t, "blah-%{[observer.version]}-blah", fmt.Sprintf("blah-%s-blah", version.Version))
	})
}

var validSourcemap, _ = os.ReadFile("../../testdata/sourcemap/bundle.js.map")

func TestStoreUsesRUMElasticsearchConfig(t *testing.T) {
	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		m := sourcemapHit(validSourcemap)

		b, err := json.Marshal(m)
		require.NoError(t, err)

		w.Write(b)
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	cfg.RumConfig.Enabled = true
	cfg.RumConfig.SourceMapping.Enabled = true
	cfg.RumConfig.SourceMapping.ESConfig = elasticsearch.DefaultConfig()
	cfg.RumConfig.SourceMapping.ESConfig.Hosts = []string{ts.URL}

	fetcher, cancel, err := newSourcemapFetcher(
		cfg.RumConfig.SourceMapping,
		nil, elasticsearch.NewClient,
	)
	require.NoError(t, err)
	defer cancel()
	// Check that the provided rum elasticsearch config was used and
	// Fetch() goes to the test server.
	c, err := fetcher.Fetch(context.Background(), "app", "1.0", "/bundle/path")
	require.NoError(t, err)
	require.NotNil(t, c)

	assert.True(t, called)
}

func sourcemapHit(sourcemap []byte) map[string]interface{} {
	b := &bytes.Buffer{}

	z := zlib.NewWriter(b)
	z.Write(sourcemap)
	z.Close()

	s := base64.StdEncoding.EncodeToString(b.Bytes())

	hits := map[string]interface{}{
		"_source": map[string]interface{}{
			"content": s,
		},
	}

	resultHits := map[string]interface{}{
		"total": map[string]interface{}{
			"value": 1,
		},
	}
	resultHits["hits"] = []map[string]interface{}{hits}
	result := map[string]interface{}{"hits": resultHits}
	return result
}

func TestQueryClusterUUIDRegistriesExist(t *testing.T) {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	stateRegistry.Clear()
	defer stateRegistry.Clear()

	elasticsearchRegistry := stateRegistry.NewRegistry("outputs.elasticsearch")
	monitoring.NewString(elasticsearchRegistry, "cluster_uuid")

	const clusterUUID = "abc123"
	client := newMockClusterUUIDClient(t, clusterUUID)

	err := queryClusterUUID(context.Background(), client)
	require.NoError(t, err)

	fs := monitoring.CollectFlatSnapshot(elasticsearchRegistry, monitoring.Full, false)
	assert.Equal(t, clusterUUID, fs.Strings["cluster_uuid"])
}

func TestQueryClusterUUIDRegistriesDoNotExist(t *testing.T) {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	stateRegistry.Clear()
	defer stateRegistry.Clear()

	const clusterUUID = "abc123"
	client := newMockClusterUUIDClient(t, clusterUUID)

	err := queryClusterUUID(context.Background(), client)
	require.NoError(t, err)

	elasticsearchRegistry := stateRegistry.GetRegistry("outputs.elasticsearch")
	require.NotNil(t, elasticsearchRegistry)

	fs := monitoring.CollectFlatSnapshot(elasticsearchRegistry, monitoring.Full, false)
	assert.Equal(t, clusterUUID, fs.Strings["cluster_uuid"])
}

func newMockClusterUUIDClient(t testing.TB, clusterUUID string) *elasticsearch.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Write([]byte(fmt.Sprintf(`{"cluster_uuid":"%s"}`, clusterUUID)))
	}))
	t.Cleanup(srv.Close)

	config := elasticsearch.DefaultConfig()
	config.Hosts = []string{srv.URL}
	client, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	return client
}
