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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var validSourcemap, _ = os.ReadFile("../../testdata/sourcemap/bundle.js.map")

func TestStoreUsesRUMElasticsearchConfig(t *testing.T) {
	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Write(validSourcemap)
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	cfg.RumConfig.Enabled = true
	cfg.RumConfig.SourceMapping.Enabled = true
	cfg.RumConfig.SourceMapping.ESConfig = elasticsearch.DefaultConfig()
	cfg.RumConfig.SourceMapping.ESConfig.Hosts = []string{ts.URL}

	fetcher, err := newSourcemapFetcher(
		cfg.RumConfig.SourceMapping, nil,
		nil, elasticsearch.NewClient,
	)
	require.NoError(t, err)
	// Check that the provided rum elasticsearch config was used and
	// Fetch() goes to the test server.
	_, err = fetcher.Fetch(context.Background(), "app", "1.0", "/bundle/path")
	require.NoError(t, err)

	assert.True(t, called)
}

func TestQueryClusterUUIDRegistriesExist(t *testing.T) {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	stateRegistry.Clear()
	defer stateRegistry.Clear()

	elasticsearchRegistry := stateRegistry.NewRegistry("outputs.elasticsearch")
	monitoring.NewString(elasticsearchRegistry, "cluster_uuid")

	ctx := context.Background()
	clusterUUID := "abc123"
	client := &mockClusterUUIDClient{ClusterUUID: clusterUUID}
	err := queryClusterUUID(ctx, client)
	require.NoError(t, err)

	fs := monitoring.CollectFlatSnapshot(elasticsearchRegistry, monitoring.Full, false)
	assert.Equal(t, clusterUUID, fs.Strings["cluster_uuid"])
}

func TestQueryClusterUUIDRegistriesDoNotExist(t *testing.T) {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	stateRegistry.Clear()
	defer stateRegistry.Clear()

	ctx := context.Background()
	clusterUUID := "abc123"
	client := &mockClusterUUIDClient{ClusterUUID: clusterUUID}
	err := queryClusterUUID(ctx, client)
	require.NoError(t, err)

	elasticsearchRegistry := stateRegistry.GetRegistry("outputs.elasticsearch")
	require.NotNil(t, elasticsearchRegistry)

	fs := monitoring.CollectFlatSnapshot(elasticsearchRegistry, monitoring.Full, false)
	assert.Equal(t, clusterUUID, fs.Strings["cluster_uuid"])
}

type mockClusterUUIDClient struct {
	ClusterUUID string `json:"cluster_uuid"`
}

func (c *mockClusterUUIDClient) Perform(r *http.Request) (*http.Response, error) {
	m, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	resp := &http.Response{
		StatusCode: 200,
		Body:       &mockReadCloser{bytes.NewReader(m)},
		Request:    r,
	}
	return resp, nil
}

func (c *mockClusterUUIDClient) NewBulkIndexer(_ elasticsearch.BulkIndexerConfig) (elasticsearch.BulkIndexer, error) {
	return nil, nil
}

type mockReadCloser struct{ r io.Reader }

func (r *mockReadCloser) Read(p []byte) (int, error) { return r.r.Read(p) }
func (r *mockReadCloser) Close() error               { return nil }
