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

package beater_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	_ "github.com/elastic/beats/v7/libbeat/outputs/console"
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/internal/beater/api"
	"github.com/elastic/apm-server/internal/beater/beatertest"
	"github.com/elastic/apm-server/internal/beater/config"
)

func TestServerOk(t *testing.T) {
	srv := beatertest.NewServer(t)
	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	res, err := srv.Client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
}

func TestServerRoot(t *testing.T) {
	srv := beatertest.NewServer(t)
	rootRequest := func(path string, accept *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if accept != nil {
			req.Header.Add("Accept", *accept)
		}
		res, err := srv.Client.Do(req)
		assert.NoError(t, err)
		return res
	}

	checkResponse := func(hasOk bool) func(t *testing.T, res *http.Response) {
		return func(t *testing.T, res *http.Response) {
			b, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			rsp := string(b)
			assert.Contains(t, rsp, "build_date")
			assert.Contains(t, rsp, "build_sha")
			assert.Contains(t, rsp, "version")
		}
	}

	jsonContent := "application/json"
	plain := "text/plain; charset=utf-8"
	testCases := []struct {
		path              string
		accept            *string
		expectStatus      int
		expectContentType string
		assertions        func(t *testing.T, res *http.Response)
	}{
		{path: "/", expectStatus: http.StatusOK, expectContentType: plain, assertions: checkResponse(false)},
		{path: "/", accept: &jsonContent, expectStatus: http.StatusOK, expectContentType: jsonContent, assertions: checkResponse(true)},
		{path: "/foo", expectStatus: http.StatusNotFound, expectContentType: plain},
		{path: "/foo", accept: &jsonContent, expectStatus: http.StatusNotFound, expectContentType: jsonContent},
	}
	for _, testCase := range testCases {
		res := rootRequest(testCase.path, testCase.accept)
		assert.Equal(t, testCase.expectStatus, res.StatusCode)
		assert.Equal(t, testCase.expectContentType, res.Header.Get("Content-Type"))
		if testCase.assertions != nil {
			testCase.assertions(t, res)
		}
		assert.NoError(t, res.Body.Close())
	}
}

func TestServerRootWithToken(t *testing.T) {
	token := "verysecret"
	badToken := "Verysecret"
	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server.auth.secret_token": token,
	})))

	rootRequest := func(token *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if token != nil {
			req.Header.Add("Authorization", "Bearer "+*token)
		}
		res, err := srv.Client.Do(req)
		require.NoError(t, err)
		return res
	}

	noToken := body(t, rootRequest(nil))
	withToken := body(t, rootRequest(&token))
	assert.NotEqual(t, token, badToken)
	withBadToken := body(t, rootRequest(&badToken))
	assert.Equal(t, 0, len(noToken), noToken)
	assert.True(t, len(withToken) > 0, withToken)
	assert.NotEqual(t, noToken, withToken)
	assert.Equal(t, noToken, withBadToken)
}

func TestServerTcpNoPort(t *testing.T) {
	// possibly flaky but worth it
	// try to connect to localhost:DefaultPort
	// if connection succeeds, port is in use and skip test
	// if it fails, make sure it is because connection refused
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", config.DefaultPort), 2*time.Second); err == nil {
		conn.Close()
		t.Skipf("default port is in use")
	} else {
		if e, ok := err.(*net.OpError); !ok || e.Op != "dial" {
			// failed for some other reason, not connection refused
			t.Error(err)
		}
	}

	srv := beatertest.NewServer(t, beatertest.WithConfig(
		agentconfig.MustNewConfigFrom(`{"apm-server.host": "localhost"}`)),
	)
	rsp, err := srv.Client.Get(srv.URL + api.RootPath)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerOkUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	addr := filepath.Join(t.TempDir(), "apm-server.sock")
	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server.host": "unix:" + addr,
	})))

	rsp, err := srv.Client.Get(srv.URL + api.RootPath)
	assert.NoError(t, err)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerHealth(t *testing.T) {
	srv := beatertest.NewServer(t)
	req, err := http.NewRequest(http.MethodGet, srv.URL+api.RootPath, nil)
	require.NoError(t, err)
	rsp, err := srv.Client.Do(req)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerRumSwitch(t *testing.T) {
	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server.rum": map[string]interface{}{
			"enabled":       true,
			"allow_origins": []string{"*"},
		},
	})))

	req, err := http.NewRequest(http.MethodPost, srv.URL+api.IntakeRUMPath, bytes.NewReader(testData))
	require.NoError(t, err)
	rsp, err := srv.Client.Do(req)
	if assert.NoError(t, err) {
		assert.NotEqual(t, http.StatusForbidden, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerCORS(t *testing.T) {
	tests := []struct {
		expectedStatus int
		origin         string
		allowedOrigins []string
	}{
		{
			expectedStatus: http.StatusForbidden,
			origin:         "http://www.example.com",
			allowedOrigins: []string{"http://notmydomain.com", "http://neitherthisone.com"},
		},
		{
			expectedStatus: http.StatusForbidden,
			origin:         "http://www.example.com",
			allowedOrigins: []string{""},
		},
		{
			expectedStatus: http.StatusForbidden,
			origin:         "http://www.example.com",
			allowedOrigins: []string{"example.com"},
		},
		{
			expectedStatus: http.StatusAccepted,
			origin:         "whatever",
			allowedOrigins: []string{"http://notmydomain.com", "*"},
		},
		{
			expectedStatus: http.StatusAccepted,
			origin:         "http://www.example.co.uk",
			allowedOrigins: []string{"http://*.example.co*"},
		},
		{
			expectedStatus: http.StatusAccepted,
			origin:         "https://www.example.com",
			allowedOrigins: []string{"http://*example.com", "https://*example.com"},
		},
	}

	for idx, test := range tests {
		t.Run(fmt.Sprint(idx), func(t *testing.T) {
			srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
				"apm-server.rum": map[string]interface{}{
					"enabled":       true,
					"allow_origins": test.allowedOrigins,
				},
			})))

			req, err := http.NewRequest(http.MethodPost, srv.URL+api.IntakeRUMPath, bytes.NewReader(testData))
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Content-Type", "application/x-ndjson")
			assert.NoError(t, err)
			rsp, err := srv.Client.Do(req)
			if assert.NoError(t, err) {
				assert.Equal(t, test.expectedStatus, rsp.StatusCode, fmt.Sprintf("Failed at idx %v; %s", idx,
					body(t, rsp)))
			}

		})
	}
}

func TestServerNoContentType(t *testing.T) {
	srv := beatertest.NewServer(t)
	req := makeTransactionRequest(t, srv.URL)
	rsp, err := srv.Client.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, http.StatusAccepted, rsp.StatusCode)
}

func TestServerJaegerGRPC(t *testing.T) {
	srv := beatertest.NewServer(t)
	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	conn, err := grpc.Dial(baseURL.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	result, err := client.PostSpans(context.Background(), &api_v2.PostSpansRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestServerOTLPGRPC(t *testing.T) {
	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server.auth.secret_token": "abc123",
	})))

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	invokeExport := func(ctx context.Context, conn *grpc.ClientConn) error {
		// We can't use go.opentelemetry.io/otel, as it has its own generated protobuf packages
		// which which conflict with opentelemetry-collector's. Instead, use the types registered
		// by the opentelemetry-collector packages.
		requestType := proto.MessageType("opentelemetry.proto.collector.trace.v1.ExportTraceServiceRequest")
		responseType := proto.MessageType("opentelemetry.proto.collector.trace.v1.ExportTraceServiceResponse")
		request := reflect.New(requestType.Elem()).Interface()
		response := reflect.New(responseType.Elem()).Interface()
		return conn.Invoke(ctx, "/opentelemetry.proto.collector.trace.v1.TraceService/Export", request, response)
	}

	conn, err := grpc.Dial(baseURL.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	ctx := context.Background()
	err = invokeExport(ctx, conn)
	assert.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("Authorization", "Bearer abc123"))
	err = invokeExport(ctx, conn)
	assert.NoError(t, err)
}

func TestServerWaitForIntegrationKibana(t *testing.T) {
	var requests int64
	requestCh := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":{"number":"1.2.3"}}`))
	})
	mux.HandleFunc("/api/fleet/epm/packages/apm", func(w http.ResponseWriter, r *http.Request) {
		switch atomic.AddInt64(&requests, 1) {
		case 1:
			w.WriteHeader(500)
		case 2:
			fmt.Fprintln(w, `{"response":{"status":"not_installed"}}`)
		case 3:
			fmt.Fprintln(w, `{"response":{"status":"installed"}}`)
		}
		select {
		case requestCh <- struct{}{}:
		case <-r.Context().Done():
		}
	})
	kibanaServer := httptest.NewServer(mux)
	defer kibanaServer.Close()

	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"wait_ready_interval":               "100ms",
			"kibana.enabled":                    true,
			"kibana.host":                       kibanaServer.URL,
			"data_streams.wait_for_integration": true,
		},
	})))

	timeout := time.After(10 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-requestCh:
		case <-timeout:
			t.Fatal("timed out waiting for request")
		}
	}

	// TODO(axw) there _should_ be just 2 logs, but there might be an initial
	// log message due to the Kibana client connecting asynchronously. We should
	// update internal/kibana to remove the async behaviour.
	logs := srv.Logs.FilterMessageSnippet("please install the apm integration")
	assert.NotZero(t, logs.Len())

	select {
	case <-requestCh:
		t.Fatal("unexpected request")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServerWaitForIntegrationElasticsearch(t *testing.T) {
	var mu sync.Mutex
	var tracesRequests int
	tracesRequestsCh := make(chan int)
	bulkCh := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		// We must send a valid JSON response for the libbeat
		// elasticsearch client to send bulk requests.
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	mux.HandleFunc("/_index_template/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		template := path.Base(r.URL.Path)
		if template == "traces-apm" {
			tracesRequests++
			if tracesRequests == 1 {
				w.WriteHeader(404)
			}
			tracesRequestsCh <- tracesRequests
		}
	})
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		select {
		case bulkCh <- struct{}{}:
		default:
		}
	})
	elasticsearchServer := httptest.NewServer(mux)
	defer elasticsearchServer.Close()

	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"wait_ready_interval":               "100ms",
			"data_streams.wait_for_integration": true,
		},
		"output.elasticsearch": map[string]interface{}{
			"hosts":       []string{elasticsearchServer.URL},
			"backoff":     map[string]interface{}{"init": "10ms", "max": "10ms"},
			"max_retries": 1000,
		},
	})))

	// Send some events to the server. They should be accepted and enqueued.
	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := srv.Client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Healthcheck should report that the server is not publish-ready.
	resp, err = srv.Client.Get(srv.URL + api.RootPath)
	require.NoError(t, err)
	out := decodeJSONMap(t, resp.Body)
	resp.Body.Close()
	assert.Equal(t, false, out["publish_ready"])

	// Indexing should be blocked until we receive from tracesRequestsCh.
	select {
	case <-bulkCh:
		t.Fatal("unexpected bulk request")
	case <-time.After(50 * time.Millisecond):
	}

	timeout := time.After(10 * time.Second)
	var done bool
	for !done {
		select {
		case n := <-tracesRequestsCh:
			done = n == 2
		case <-timeout:
			t.Fatal("timed out waiting for request")
		}
	}

	// libbeat should keep retrying, and finally succeed now it is unblocked.
	select {
	case <-bulkCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for bulk request")
	}

	logs := srv.Logs.FilterMessageSnippet("please install the apm integration")
	assert.Len(t, logs.All(), 1, "couldn't find remediation message logs")

	// Healthcheck should now report that the server is publish-ready.
	resp, err = srv.Client.Get(srv.URL + api.RootPath)
	require.NoError(t, err)
	out = decodeJSONMap(t, resp.Body)
	resp.Body.Close()
	assert.Equal(t, true, out["publish_ready"])
}

func TestServerFailedPreconditionDoesNotIndex(t *testing.T) {
	bulkCh := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		// We must send a valid JSON response for the libbeat
		// elasticsearch client to send bulk requests.
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	mux.HandleFunc("/_index_template/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		select {
		case bulkCh <- struct{}{}:
		default:
		}
	})
	elasticsearchServer := httptest.NewServer(mux)
	defer elasticsearchServer.Close()

	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"wait_ready_interval":               "100ms",
			"data_streams.wait_for_integration": true,
		},
		"output.elasticsearch.hosts": []string{elasticsearchServer.URL},
	})))

	// Send some events to the server. They should be accepted and enqueued.
	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := srv.Client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Healthcheck should report that the server is not publish-ready.
	resp, err = srv.Client.Get(srv.URL + api.RootPath)
	require.NoError(t, err)
	out := decodeJSONMap(t, resp.Body)
	resp.Body.Close()
	assert.Equal(t, false, out["publish_ready"])

	// Stop the server.
	srv.Close()

	// No documents should be indexed.
	select {
	case <-bulkCh:
		t.Fatal("unexpected bulk request")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTailSamplingPlatinumLicense(t *testing.T) {
	bulkCh := make(chan struct{}, 1)
	licenseReq := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		// We must send a valid JSON response for the libbeat
		// elasticsearch client to send bulk requests.
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})
	mux.HandleFunc("/_license", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case licenseReq <- struct{}{}:
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		fmt.Fprintln(w, `{"license":{"uid":"cbff45e7-c553-41f7-ae4f-9205eabd80xx","type":"basic","status":"active"}}`)
	})
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		select {
		case bulkCh <- struct{}{}:
		default:
		}
	})
	elasticsearchServer := httptest.NewServer(mux)
	defer elasticsearchServer.Close()

	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"wait_ready_interval": "100ms",
			"sampling.tail": map[string]interface{}{
				"enabled":  true,
				"policies": []map[string]interface{}{{"sample_rate": 0.1}},
			},
		},
		"output.elasticsearch.hosts": []string{elasticsearchServer.URL},
	})))

	// Send some events to the server. They should be accepted and enqueued.
	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := srv.Client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Wait for two license queries, after which the server should definitely
	// have processed the first license response, and should have logged an
	// error message about the license level being invalid.
	for i := 0; i < 2; i++ {
		<-licenseReq
	}
	logs := srv.Logs.FilterMessageSnippet("invalid license level Basic: tail-based sampling requires license level Platinum")
	assert.NotZero(t, logs.Len())

	// Healthcheck should report that the server is not publish-ready.
	resp, err = srv.Client.Get(srv.URL + api.RootPath)
	require.NoError(t, err)
	out := decodeJSONMap(t, resp.Body)
	resp.Body.Close()
	assert.Equal(t, false, out["publish_ready"])

	// Stop the server.
	srv.Close()

	// No documents should be indexed.
	select {
	case <-bulkCh:
		t.Fatal("unexpected bulk request")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestServerElasticsearchOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		// We must send a valid JSON response for the libbeat
		// elasticsearch client to send bulk requests.
		fmt.Fprintln(w, `{"version":{"number":"1.2.3"}}`)
	})

	done := make(chan struct{})
	bulkCh := make(chan *http.Request, 1)
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		select {
		case bulkCh <- r:
		default:
		}
		<-done // block all requests from completing until test is done
	})
	elasticsearchServer := httptest.NewServer(mux)
	defer elasticsearchServer.Close()
	defer close(done)

	// Pre-create the libbeat registry with some variables that should not
	// be reported, as we define our own libbeat metrics registry.
	monitoring.Default.Remove("libbeat.whatever")
	monitoring.NewInt(monitoring.Default, "libbeat.whatever")

	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(map[string]interface{}{
		"output.elasticsearch": map[string]interface{}{
			"hosts":        []string{elasticsearchServer.URL},
			"flush_bytes":  "1kb", // test data is >1kb
			"backoff":      map[string]interface{}{"init": "1ms", "max": "1ms"},
			"max_retries":  0,
			"max_requests": 10,
		},
	})))

	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := srv.Client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	select {
	case r := <-bulkCh:
		userAgent := r.UserAgent()
		assert.True(t, strings.Contains(userAgent, "Elastic-APM-Server"), userAgent)
		assert.True(t, strings.Contains(userAgent, "go-elasticsearch"), userAgent)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for bulk request")
	}

	snapshot := monitoring.CollectStructSnapshot(monitoring.Default.GetRegistry("libbeat"), monitoring.Full, false)
	assert.Equal(t, map[string]interface{}{
		"output": map[string]interface{}{
			"events": map[string]interface{}{
				"acked":   int64(0),
				"active":  int64(5),
				"batches": int64(0),
				"failed":  int64(0),
				"toomany": int64(0),
				"total":   int64(5),
			},
			"type": "elasticsearch",
			"write": map[string]interface{}{
				// _bulk requests haven't completed, so bytes flushed won't have been updated.
				"bytes": int64(0),
			},
		},
		"pipeline": map[string]interface{}{
			"events": map[string]interface{}{
				"total": int64(5),
			},
		},
	}, snapshot)

	snapshot = monitoring.CollectStructSnapshot(monitoring.Default.GetRegistry("output"), monitoring.Full, false)
	assert.Equal(t, map[string]interface{}{
		"elasticsearch": map[string]interface{}{
			"bulk_requests": map[string]interface{}{
				"available": int64(9),
				"completed": int64(0),
			},
			"indexers": map[string]interface{}{
				"active":    int64(1),
				"destroyed": int64(0),
				"created":   int64(0),
			},
		},
	}, snapshot)
}

func TestServerPProf(t *testing.T) {
	srv := beatertest.NewServer(t, beatertest.WithConfig(agentconfig.MustNewConfigFrom(`{"apm-server.pprof.enabled": true}`)))
	for _, path := range []string{
		"/debug/pprof",
		"/debug/pprof/goroutine",
		"/debug/pprof/cmdline",
	} {
		resp, err := http.Get(srv.URL + path)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, path)
	}
}

func TestWrapServer(t *testing.T) {
	escfg, docs := beatertest.ElasticsearchOutputConfig(t)
	srv := beatertest.NewServer(t, beatertest.WithConfig(escfg), beatertest.WithWrapServer(
		func(args beater.ServerParams, runServer beater.RunServerFunc) (beater.ServerParams, beater.RunServerFunc, error) {
			origBatchProcessor := args.BatchProcessor
			args.BatchProcessor = modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
				for i := range *batch {
					event := (*batch)[i]
					if event.Type() != modelpb.TransactionEventType {
						continue
					}
					// Add a label to test that everything
					// goes through the wrapped reporter.
					if event.Labels == nil {
						event.Labels = make(modelpb.Labels)
					}
					modelpb.Labels(event.Labels).Set("wrapped_reporter", "true")
				}
				return origBatchProcessor.ProcessBatch(ctx, batch)
			})
			return args, runServer, nil
		},
	))

	req := makeTransactionRequest(t, srv.URL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	res, err := srv.Client.Do(req)
	assert.NoError(t, err)
	res.Body.Close()

	doc := <-docs
	var out map[string]any
	require.NoError(t, json.Unmarshal(doc, &out))
	require.Contains(t, out, "labels")
	require.Contains(t, out["labels"], "wrapped_reporter")
	require.Equal(t, "true", out["labels"].(map[string]any)["wrapped_reporter"])
}

var testData = func() []byte {
	b, err := os.ReadFile("../../testdata/intake-v2/transactions.ndjson")
	if err != nil {
		panic(err)
	}
	return b
}()

func makeTransactionRequest(t *testing.T, baseURL string) *http.Request {
	req, err := http.NewRequest(http.MethodPost, baseURL+api.IntakePath, bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	return req
}

func decodeJSONMap(t *testing.T, r io.Reader) map[string]interface{} {
	out := make(map[string]interface{})
	err := json.NewDecoder(r).Decode(&out)
	require.NoError(t, err)
	return out
}

func body(t *testing.T, response *http.Response) string {
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return string(body)
}
