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

package sourcemap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libkibana "github.com/elastic/elastic-agent-libs/kibana"

	"github.com/elastic/apm-server/internal/kibana"
)

func TestKibanaFetcher(t *testing.T) {
	fetcher := newTestKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"artifacts": []interface{}{
				map[string]interface{}{
					"type": "not_a_sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "service_name",
						"serviceVersion": "service_version",
						"bundleFilepath": "http://another_host:456/path",
						"sourceMap":      "invalid_sourcemap",
					},
				},
				map[string]interface{}{
					"type": "sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "non_matching_service_name",
						"serviceVersion": "service_version",
						"bundleFilepath": "http://another_host:456/path",
						"sourceMap":      "invalid_sourcemap",
					},
				},
				map[string]interface{}{
					"type": "sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "service_name",
						"serviceVersion": "non_matching_service_version",
						"bundleFilepath": "http://another_host:456/path",
						"sourceMap":      "invalid_sourcemap",
					},
				},
				map[string]interface{}{
					"type": "sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "service_name",
						"serviceVersion": "service_version",
						"bundleFilepath": "http://another_host:456/non_matching_path",
						"sourceMap":      "invalid_sourcemap",
					},
				},
				map[string]interface{}{
					"type": "sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "service_name",
						"serviceVersion": "service_version",
						"bundleFilepath": "http://another_host:456/path",
						"sourceMap":      json.RawMessage(validSourcemap),
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	})
	consumer, err := fetcher.Fetch(context.Background(), "service_name", "service_version", "http://host:123/path")
	require.NoError(t, err)
	assert.NotNil(t, consumer)
}

func TestKibanaFetcherInvalidSourcemap(t *testing.T) {
	fetcher := newTestKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"artifacts": []interface{}{
				map[string]interface{}{
					"type": "sourcemap",
					"body": map[string]interface{}{
						"serviceName":    "service_name",
						"serviceVersion": "service_version",
						"bundleFilepath": "http://another_host:456/path",
						"sourceMap":      "invalid_sourcemap",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	})
	consumer, err := fetcher.Fetch(context.Background(), "service_name", "service_version", "http://host:123/path")
	require.Error(t, err)
	assert.EqualError(t, err, "sourcemap malformed: json: cannot unmarshal string into Go value of type sourcemap.v3")
	assert.Nil(t, consumer)
}

func TestKibanaFetcherNotFound(t *testing.T) {
	fetcher := newTestKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"artifacts":[]}`)
	})
	consumer, err := fetcher.Fetch(context.Background(), "service_name", "service_version", "http://host:123/path")
	require.NoError(t, err)
	assert.Nil(t, consumer)
}

func TestKibanaFetcherServerError(t *testing.T) {
	fetcher := newTestKibanaFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("terrible things"))
	})
	consumer, err := fetcher.Fetch(context.Background(), "service_name", "service_version", "http://host:123/path")
	require.Error(t, err)
	assert.EqualError(t, err, "failed to query source maps (500 Internal Server Error): terrible things")
	assert.Nil(t, consumer)
}

func newTestKibanaFetcher(t testing.TB, h http.HandlerFunc) Fetcher {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/apm/sourcemaps", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	kibanaClient, err := kibana.NewClient(libkibana.ClientConfig{
		Host: srv.Listener.Addr().String(),
	})
	require.NoError(t, err)
	return NewKibanaFetcher(kibanaClient)
}
