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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/elastic/elastic-agent-libs/logp/logptest"

	"github.com/elastic/apm-server/internal/elasticsearch"
)

func TestMetadataFetcher(t *testing.T) {
	defaultID := metadata{
		identifier: identifier{
			name:    "app",
			version: "1.0",
			path:    "/bundle/path",
		},
		contentHash: "foo",
	}

	defaultSearchResponse := func(w http.ResponseWriter, r *http.Request) {
		m := sourcemapSearchResponseBody([]metadata{defaultID})
		w.Write(m)
	}

	testCases := []struct {
		name              string
		pingStatus        int
		pingUnreachable   bool
		searchUnreachable bool
		searchReponse     func(http.ResponseWriter, *http.Request)
		expectErr         bool
		expectID          bool
	}{
		{
			name:          "200",
			pingStatus:    http.StatusOK,
			searchReponse: defaultSearchResponse,
			expectID:      true,
		}, {
			name:              "search unreachable",
			pingStatus:        http.StatusOK,
			searchUnreachable: true,
			expectErr:         true,
			expectID:          false,
		}, {
			name:       "init error",
			pingStatus: http.StatusOK,
			searchReponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusGatewayTimeout)
			},
			expectErr: true,
			expectID:  false,
		}, {
			name:       "malformed response",
			pingStatus: http.StatusOK,
			searchReponse: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("foo"))
			},
			expectErr: true,
			expectID:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			waitCh := make(chan struct{})

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				switch r.URL.Path {
				case "/.apm-source-map/_search":
					if tc.searchUnreachable {
						<-waitCh
						return
					}
					require.NotNil(t, tc.searchReponse, "nil searchReponse, possible unexpected request")
					// search request from the metadata fetcher
					tc.searchReponse(w, r)
				case "/_search/scroll":
					scrollIDValue := r.URL.Query().Get("scroll_id")
					assert.Equal(t, scrollID, scrollIDValue)
					w.Write([]byte(`{}`))
				case "/_search/scroll/" + scrollID:
					assert.Equal(t, r.Method, http.MethodDelete)
				default:
					w.WriteHeader(http.StatusTeapot)
					t.Fatalf("unhandled request path: %s", r.URL.Path)
				}
			}))
			defer ts.Close()

			esConfig := elasticsearch.DefaultConfig()
			esConfig.Hosts = []string{ts.URL}

			esClient, err := elasticsearch.NewClient(elasticsearch.ClientParams{
				Config: esConfig,
				Logger: logptest.NewTestingLogger(t, ""),
			})
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			exporter := &manualExporter{}
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))
			fetcher, invalidationChan := NewMetadataFetcher(ctx, esClient, ".apm-source-map", tp, logptest.NewTestingLogger(t, ""))

			<-fetcher.ready()
			if tc.expectErr {
				assert.Error(t, fetcher.err())
			} else {
				assert.NoError(t, fetcher.err())
			}

			_, ok := fetcher.getID(defaultID.identifier)
			assert.Equal(t, tc.expectID, ok)

			close(waitCh)
			tp.ForceFlush(ctx)

			assert.Greater(t, len(exporter.spans), 1)

			cancel()
			// wait for invalidationChan to be closed so
			// the background goroutine created by NewMetadataFetcher is done
			for range invalidationChan {
			}
		})
	}
}

type manualExporter struct {
	spans []sdktrace.ReadOnlySpan
}

func (e *manualExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *manualExporter) Shutdown(ctx context.Context) error {
	return nil
}

type metadata struct {
	identifier
	contentHash string
}

func TestInvalidation(t *testing.T) {
	defaultID := metadata{
		identifier: identifier{
			name:    "app",
			version: "1.0",
			path:    "/bundle/path",
		},
		contentHash: "foo",
	}

	defaultSearchResponse := func(w http.ResponseWriter, r *http.Request) {
		m := sourcemapSearchResponseBody([]metadata{defaultID})
		w.Write(m)
	}

	testCases := []struct {
		name                 string
		set                  map[identifier]string
		alias                map[identifier]*identifier
		searchReponse        func(http.ResponseWriter, *http.Request)
		expectedInvalidation []identifier
		expectedset          map[identifier]string
		expectedalias        map[identifier]*identifier
	}{
		{
			name:                 "hash changed",
			set:                  map[identifier]string{defaultID.identifier: "bar"},
			searchReponse:        defaultSearchResponse,
			expectedInvalidation: []identifier{defaultID.identifier},
			expectedset:          map[identifier]string{defaultID.identifier: "foo"},
			expectedalias:        map[identifier]*identifier{},
		}, {
			name: "sourcemap deleted",
			set:  map[identifier]string{defaultID.identifier: "bar"},
			searchReponse: func(w http.ResponseWriter, r *http.Request) {
				m := sourcemapSearchResponseBody([]metadata{})
				w.Write(m)
			},
			expectedInvalidation: []identifier{defaultID.identifier},
			expectedset:          map[identifier]string{},
			expectedalias:        map[identifier]*identifier{},
		}, {
			name: "update ok",
			set:  map[identifier]string{{name: "example", version: "1.0", path: "/"}: "bar"},
			searchReponse: func(w http.ResponseWriter, r *http.Request) {
				bar := metadata{
					identifier: identifier{
						name:    "example",
						version: "1.0",
						path:    "/",
					},
					contentHash: "bar",
				}
				m := sourcemapSearchResponseBody([]metadata{defaultID, bar})
				w.Write(m)
			},
			expectedset:   map[identifier]string{defaultID.identifier: "foo", {name: "example", version: "1.0", path: "/"}: "bar"},
			expectedalias: map[identifier]*identifier{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := make(chan struct{})

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-c:
				case <-time.After(1 * time.Second):
					t.Fatalf("timeout out waiting for channel")
				}
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				switch r.URL.Path {
				case "/.apm-source-map":
					// ping request from the metadata fetcher.
					// Send a status ok
					w.WriteHeader(http.StatusOK)
				case "/.apm-source-map/_search":
					// search request from the metadata fetcher
					tc.searchReponse(w, r)
				case "/_search/scroll":
					scrollIDValue := r.URL.Query().Get("scroll_id")
					assert.Equal(t, scrollID, scrollIDValue)
					w.Write([]byte(`{}`))
				case "/_search/scroll/" + scrollID:
					assert.Equal(t, r.Method, http.MethodDelete)
				default:
					w.WriteHeader(http.StatusTeapot)
					t.Fatalf("unhandled request path: %s", r.URL.Path)
				}
			}))
			defer ts.Close()

			esConfig := elasticsearch.DefaultConfig()
			esConfig.Hosts = []string{ts.URL}

			esClient, err := elasticsearch.NewClient(elasticsearch.ClientParams{
				Config: esConfig,
				Logger: logptest.NewTestingLogger(t, ""),
			})
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			fetcher, invalidationChan := NewMetadataFetcher(ctx, esClient, ".apm-source-map", noop.NewTracerProvider(), logptest.NewTestingLogger(t, ""))

			invCh := make(chan struct{})
			go func() {
				defer close(invCh)
				i, ok := <-invalidationChan
				if !ok {
					return
				}
				if tc.expectedInvalidation == nil {
					t.Errorf("unexpected invalidation: %v", i)
				} else {
					assert.Equal(t, tc.expectedInvalidation, i)
				}
			}()

			esf := fetcher.(*MetadataESFetcher)
			if tc.set != nil {
				esf.set = tc.set
			}
			if tc.alias != nil {
				esf.alias = tc.alias
			}
			close(c)

			<-fetcher.ready()

			assert.NoError(t, fetcher.err())

			assert.Equal(t, esf.set, tc.expectedset)
			assert.Equal(t, esf.alias, tc.expectedalias)

			if tc.expectedInvalidation != nil {
				select {
				case <-invCh:
				case <-time.After(50 * time.Millisecond):
					t.Fatal("timed out waiting for invalidations")
				}
			}

			cancel()
			<-invCh
			// wait for invalidationChan to be closed so
			// the background goroutine created by NewMetadataFetcher is done
			for range invalidationChan {
			}
		})
	}
}

const scrollID = "FGluY2x1ZGVfY29udGV4dF91dWlkDXF1ZXJ5QW5kRmV0Y2gBFkJUT0Z5bFUtUXRXM3NTYno0dkM2MlEAAAAAAABnRBY5OUxYalAwUFFoS1NfLV9lWjlSYTRn"

func sourcemapSearchResponseBody(ids []metadata) []byte {
	m := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		m = append(m, map[string]interface{}{
			"_source": map[string]interface{}{
				"service": map[string]interface{}{
					"name":    id.name,
					"version": id.version,
				},
				"file": map[string]interface{}{
					"path": id.path,
				},
				"content_sha256": id.contentHash,
			},
		})
	}

	result := map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{
				"value": len(ids),
			},
			"hits": m,
		},
		"_scroll_id": scrollID,
	}

	data, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return data
}
