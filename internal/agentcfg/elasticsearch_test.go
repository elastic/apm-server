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

package agentcfg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

var sampleHits = []map[string]interface{}{
	{"_id": "h_KmzYQBfJ4l0GgqXgKA", "_index": ".apm-agent-configuration", "_score": 1, "_source": map[string]interface{}{"@timestamp": 1.669897543296e+12, "applied_by_agent": false, "etag": "ef12bf5e879c38e931d2894a9c90b2cb1b5fa190", "service": map[string]interface{}{"name": "first"}, "settings": map[string]interface{}{"sanitize_field_names": "foo,bar,baz", "transaction_sample_rate": "0.1"}}},
	{"_id": "hvKmzYQBfJ4l0GgqXgJt", "_index": ".apm-agent-configuration", "_score": 1, "_source": map[string]interface{}{"@timestamp": 1.669897543277e+12, "applied_by_agent": false, "etag": "2da2f86251165ccced5c5e41100a216b0c880db4", "service": map[string]interface{}{"name": "second"}, "settings": map[string]interface{}{"sanitize_field_names": "foo,bar,baz", "transaction_sample_rate": "0.1"}}},
}

func newMockElasticsearchClient(t testing.TB, handler func(http.ResponseWriter, *http.Request)) *elasticsearch.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	config := elasticsearch.DefaultConfig()
	config.Backoff.Init = time.Nanosecond
	config.Hosts = []string{srv.URL}
	client, err := elasticsearch.NewClient(elasticsearch.ClientParams{
		Config: config,
		Logger: logptest.NewTestingLogger(t, ""),
	})
	require.NoError(t, err)
	return client
}

func newElasticsearchFetcher(
	t testing.TB,
	hits []map[string]interface{},
	searchSize int,
	tp trace.TracerProvider,
) *ElasticsearchFetcher {
	var maxScore = func(hits []map[string]interface{}) interface{} {
		if len(hits) == 0 {
			return nil
		}
		return 1
	}
	var respTmpl = map[string]interface{}{
		"_scroll_id": "FGluY2x1ZGVfY29udGV4dF91dWlkDXF1ZXJ5QW5kRmV0Y2gBFkJUT0Z5bFUtUXRXM3NTYno0dkM2MlEAAAAAAABnRBY5OUxYalAwUFFoS1NfLV9lWjlSYTRn",
		"_shards":    map[string]interface{}{"failed": 0, "skipped": 0, "successful": 1, "total": 1},
		"hits": map[string]interface{}{
			"hits":      []map[string]interface{}{},
			"max_score": maxScore(hits),
			"total":     map[string]interface{}{"relation": "eq", "value": len(hits)},
		},
		"timed_out": false,
		"took":      1,
	}

	i := 0

	fetcher := NewElasticsearchFetcher(newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/_search/scroll") {
			scrollID := strings.TrimPrefix(r.URL.Path, "/_search/scroll/")
			assert.Equal(t, respTmpl["_scroll_id"], scrollID)
			return
		}
		switch r.URL.Path {
		case "/_search/scroll":
			scrollID := r.URL.Query().Get("scroll_id")
			assert.Equal(t, respTmpl["_scroll_id"], scrollID)
		case "/.apm-agent-configuration/_search":
		default:
			assert.Failf(t, "unexpected path", "path: %s", r.URL.Path)
		}
		if i < len(hits) {
			respTmpl["hits"].(map[string]interface{})["hits"] = hits[i : i+searchSize]
		} else {
			respTmpl["hits"].(map[string]interface{})["hits"] = []map[string]interface{}{}
		}

		b, err := json.Marshal(respTmpl)
		require.NoError(t, err)
		w.WriteHeader(200)
		w.Write(b)
		i += searchSize
	}), time.Second, nil, tp, metricnoop.NewMeterProvider(), logptest.NewTestingLogger(t, ""))
	fetcher.searchSize = searchSize
	return fetcher
}

type manualExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *manualExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *manualExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (e *manualExporter) payloads() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.spans
}

func TestRun(t *testing.T) {
	exporter := &manualExporter{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))

	fetcher := newElasticsearchFetcher(t, sampleHits, 2, tp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := fetcher.Run(ctx)
		assert.Equal(t, context.Canceled, err)
	}()

	assert.Eventually(t, func() bool {
		tp.ForceFlush(t.Context())
		payloads := exporter.payloads()
		return len(payloads) == 2
	}, 10*time.Second, 10*time.Millisecond)

	payloads := exporter.payloads()
	assert.Equal(t, "ElasticsearchFetcher.refreshCache", payloads[0].Name())
	assert.Equal(t, "ElasticsearchFetcher.refresh", payloads[1].Name())
	assert.Equal(t, payloads[1].SpanContext().SpanID(), payloads[0].Parent().SpanID())
}

func TestFetch(t *testing.T) {
	fetcher := newElasticsearchFetcher(t, sampleHits, 2, tracenoop.NewTracerProvider())
	err := fetcher.refreshCache(context.Background())
	require.NoError(t, err)
	require.Len(t, fetcher.cache, 2)

	result, err := fetcher.Fetch(context.Background(), Query{Service: Service{Name: "first"}, Etag: ""})
	require.NoError(t, err)
	require.Equal(t, Result{Source: Source{
		Settings: map[string]string{"sanitize_field_names": "foo,bar,baz", "transaction_sample_rate": "0.1"},
		Etag:     "ef12bf5e879c38e931d2894a9c90b2cb1b5fa190",
		Agent:    "",
	}}, result)
}

func TestRefreshCacheScroll(t *testing.T) {
	fetcher := newElasticsearchFetcher(t, sampleHits, 1, tracenoop.NewTracerProvider())
	err := fetcher.refreshCache(context.Background())
	require.NoError(t, err)
	require.Len(t, fetcher.cache, 2)
	require.Equal(t, "first", fetcher.cache[0].ServiceName)
	require.Equal(t, "second", fetcher.cache[1].ServiceName)
}

func TestFetchOnCacheNotReady(t *testing.T) {
	fetcher := newElasticsearchFetcher(t, []map[string]interface{}{}, 1, tracenoop.NewTracerProvider())

	_, err := fetcher.Fetch(context.Background(), Query{Service: Service{Name: ""}, Etag: ""})
	require.EqualError(t, err, ErrInfrastructureNotReady)

	err = fetcher.refreshCache(context.Background())
	require.NoError(t, err)

	_, err = fetcher.Fetch(context.Background(), Query{Service: Service{Name: ""}, Etag: ""})
	require.NoError(t, err)
}

type fetcherFunc func(context.Context, Query) (Result, error)

func (f fetcherFunc) Fetch(ctx context.Context, query Query) (Result, error) {
	return f(ctx, query)
}

func TestFetchUseFallback(t *testing.T) {
	fallbackFetcherCalled := false
	fallbackFetcher := fetcherFunc(func(context.Context, Query) (Result, error) {
		fallbackFetcherCalled = true
		return Result{}, nil
	})
	fetcher := NewElasticsearchFetcher(
		newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}),
		time.Second,
		fallbackFetcher,
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
		logptest.NewTestingLogger(t, ""),
	)

	fetcher.refreshCache(context.Background())
	fetcher.Fetch(context.Background(), Query{Service: Service{Name: ""}, Etag: ""})
	require.True(t, fallbackFetcherCalled)
}

func TestFetchNoFallbackInvalidESCfg(t *testing.T) {
	fetcher := NewElasticsearchFetcher(
		newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(401)
		}),
		time.Second,
		nil,
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
		logptest.NewTestingLogger(t, ""),
	)

	err := fetcher.refreshCache(context.Background())
	require.EqualError(t, err, "refresh cache elasticsearch returned status 401")
	_, err = fetcher.Fetch(context.Background(), Query{Service: Service{Name: ""}, Etag: ""})
	require.EqualError(t, err, ErrNoValidElasticsearchConfig)
}

func TestFetchNoFallback(t *testing.T) {
	fetcher := NewElasticsearchFetcher(
		newMockElasticsearchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}),
		time.Second,
		nil,
		tracenoop.NewTracerProvider(),
		metricnoop.NewMeterProvider(),
		logptest.NewTestingLogger(t, ""),
	)

	err := fetcher.refreshCache(context.Background())
	require.EqualError(t, err, "refresh cache elasticsearch returned status 500")
	_, err = fetcher.Fetch(context.Background(), Query{Service: Service{Name: ""}, Etag: ""})
	require.EqualError(t, err, ErrInfrastructureNotReady)
}
