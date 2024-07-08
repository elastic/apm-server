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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2/apmtest"
	"go.uber.org/zap"

	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/elasticsearch"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-docappender/v2"
)

func TestStoreUsesRUMElasticsearchConfig(t *testing.T) {
	initCh := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch r.URL.Path {
		case "/.apm-source-map/_search":
			// search request from the metadata fetcher
			m := sourcemapSearchResponseBody("app", "1.0", "/bundle/path")
			w.Write(m)
			close(initCh)
		default:
			w.WriteHeader(http.StatusTeapot)
			t.Fatalf("unhandled request path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	cfg := config.DefaultConfig()
	cfg.RumConfig.Enabled = true
	cfg.Kibana.Enabled = false
	cfg.RumConfig.SourceMapping.Enabled = true
	cfg.RumConfig.SourceMapping.ESConfig = elasticsearch.DefaultConfig()
	cfg.RumConfig.SourceMapping.ESConfig.Hosts = []string{ts.URL}

	_, cancel, err := newSourcemapFetcher(
		cfg.RumConfig.SourceMapping,
		nil, elasticsearch.NewClient,
		apmtest.NewRecordingTracer().Tracer,
	)
	require.NoError(t, err)
	defer cancel()

	select {
	case <-initCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for metadata fetcher init to complete")
	}
}

func sourcemapSearchResponseBody(name string, version string, bundlePath string) []byte {
	result := map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{
				"value": 1,
			},
			"hits": []map[string]interface{}{
				{
					"_source": map[string]interface{}{
						"service": map[string]interface{}{
							"name":    name,
							"version": version,
						},
						"file": map[string]interface{}{
							"path": bundlePath,
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return data
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

func TestRunnerNewDocappenderConfig(t *testing.T) {
	var tc = []struct {
		memSize         float64
		wantMaxRequests int
		wantDocBufSize  int
	}{
		{memSize: 1, wantMaxRequests: 11, wantDocBufSize: 819},
		{memSize: 2, wantMaxRequests: 13, wantDocBufSize: 1638},
		{memSize: 4, wantMaxRequests: 16, wantDocBufSize: 3276},
		{memSize: 8, wantMaxRequests: 22, wantDocBufSize: 6553},
	}
	for _, c := range tc {
		t.Run(fmt.Sprintf("default/%vgb", c.memSize), func(t *testing.T) {
			r := Runner{
				elasticsearchOutputConfig: agentconfig.NewConfig(),
				logger:                    logp.NewLogger("test"),
			}
			docCfg, esCfg, err := r.newDocappenderConfig(nil, c.memSize)
			require.NoError(t, err)
			assert.Equal(t, docappender.Config{
				Logger:                zap.New(r.logger.Core(), zap.WithCaller(true)),
				CompressionLevel:      5,
				RequireDataStream:     true,
				FlushInterval:         time.Second,
				FlushBytes:            1024 * 1024,
				MaxRequests:           c.wantMaxRequests,
				DocumentBufferSize:    c.wantDocBufSize,
				MaxDocumentRetries:    3,
				RetryOnDocumentStatus: []int{429},
			}, docCfg)
			assert.Equal(t, &elasticsearch.Config{
				Hosts:               elasticsearch.Hosts{"localhost:9200"},
				Backoff:             elasticsearch.DefaultBackoffConfig,
				Protocol:            "http",
				CompressionLevel:    5,
				Timeout:             5 * time.Second,
				MaxRetries:          3,
				MaxIdleConnsPerHost: c.wantMaxRequests,
			}, esCfg)
		})
		t.Run(fmt.Sprintf("override/%vgb", c.memSize), func(t *testing.T) {
			r := Runner{
				elasticsearchOutputConfig: agentconfig.MustNewConfigFrom(map[string]interface{}{
					"flush_bytes":    "500 kib",
					"flush_interval": "2s",
					"max_requests":   50,
				}),
				logger: logp.NewLogger("test"),
			}
			docCfg, esCfg, err := r.newDocappenderConfig(nil, c.memSize)
			require.NoError(t, err)
			assert.Equal(t, docappender.Config{
				Logger:                zap.New(r.logger.Core(), zap.WithCaller(true)),
				CompressionLevel:      5,
				RequireDataStream:     true,
				FlushInterval:         2 * time.Second,
				FlushBytes:            500 * 1024,
				MaxRequests:           50,
				DocumentBufferSize:    c.wantDocBufSize,
				MaxDocumentRetries:    3,
				RetryOnDocumentStatus: []int{429},
			}, docCfg)
			assert.Equal(t, &elasticsearch.Config{
				Hosts:               elasticsearch.Hosts{"localhost:9200"},
				Backoff:             elasticsearch.DefaultBackoffConfig,
				Protocol:            "http",
				CompressionLevel:    5,
				Timeout:             5 * time.Second,
				MaxRetries:          3,
				MaxIdleConnsPerHost: 50,
			}, esCfg)
		})
	}
}
