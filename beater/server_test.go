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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/management"
	"github.com/elastic/beats/v7/libbeat/outputs"
	pubs "github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/processing"
	"github.com/elastic/beats/v7/libbeat/publisher/queue"
	"github.com/elastic/beats/v7/libbeat/publisher/queue/memqueue"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
)

type m map[string]interface{}

func TestServerOk(t *testing.T) {
	apm, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	req := makeTransactionRequest(t, apm.baseURL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	res, err := apm.client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
}

func TestServerRoot(t *testing.T) {
	apm, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	rootRequest := func(path string, accept *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, apm.baseURL+path, nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if accept != nil {
			req.Header.Add("Accept", *accept)
		}
		res, err := apm.client.Do(req)
		assert.NoError(t, err)
		return res
	}

	checkResponse := func(hasOk bool) func(t *testing.T, res *http.Response) {
		return func(t *testing.T, res *http.Response) {
			b, err := ioutil.ReadAll(res.Body)
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
	}
}

func TestServerRootWithToken(t *testing.T) {
	token := "verysecret"
	badToken := "Verysecret"
	ucfg, err := common.NewConfigFrom(m{"secret_token": token})
	assert.NoError(t, err)
	apm, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	rootRequest := func(token *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, apm.baseURL+"/", nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if token != nil {
			req.Header.Add("Authorization", "Bearer "+*token)
		}
		res, err := apm.client.Do(req)
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
	ucfg, err := common.NewConfigFrom(map[string]interface{}{
		"host": "localhost",
	})
	assert.NoError(t, err)
	btr, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer btr.Stop()

	rsp, err := btr.client.Get(btr.baseURL + api.RootPath)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func tmpTestUnix(t *testing.T) string {
	f, err := ioutil.TempFile("", "test-apm-server")
	assert.NoError(t, err)
	addr := f.Name()
	f.Close()
	os.Remove(addr)
	return addr
}

func TestServerOkUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}

	addr := tmpTestUnix(t)
	ucfg, err := common.NewConfigFrom(map[string]interface{}{"host": "unix:" + addr})
	assert.NoError(t, err)
	btr, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer btr.Stop()

	rsp, err := btr.client.Get(btr.baseURL + api.RootPath)
	assert.NoError(t, err)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerHealth(t *testing.T) {
	apm, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	req, err := http.NewRequest(http.MethodGet, apm.baseURL+api.RootPath, nil)
	require.NoError(t, err)
	rsp, err := apm.client.Do(req)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerRumSwitch(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "allow_origins": []string{"*"}}})
	assert.NoError(t, err)
	apm, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	req, err := http.NewRequest(http.MethodPost, apm.baseURL+api.IntakeRUMPath, bytes.NewReader(testData))
	require.NoError(t, err)
	rsp, err := apm.client.Do(req)
	if assert.NoError(t, err) {
		assert.NotEqual(t, http.StatusForbidden, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerSourcemapBadConfig(t *testing.T) {
	// TODO(axw) fix this, it shouldn't be possible
	// to create config with an empty hosts list.
	t.Skip("test is broken, config is no longer invalid")

	ucfg, err := common.NewConfigFrom(
		m{"rum": m{"enabled": true, "source_mapping": m{"elasticsearch": m{"hosts": []string{}}}}},
	)
	require.NoError(t, err)
	s, err := setupServer(t, ucfg, nil, nil)
	require.Nil(t, s)
	require.Error(t, err)
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
			ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "allow_origins": test.allowedOrigins}})
			assert.NoError(t, err)
			apm, err := setupServer(t, ucfg, nil, nil)
			require.NoError(t, err)
			defer apm.Stop()

			req, err := http.NewRequest(http.MethodPost, apm.baseURL+api.IntakeRUMPath, bytes.NewReader(testData))
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Content-Type", "application/x-ndjson")
			assert.NoError(t, err)
			rsp, err := apm.client.Do(req)
			if assert.NoError(t, err) {
				assert.Equal(t, test.expectedStatus, rsp.StatusCode, fmt.Sprintf("Failed at idx %v; %s", idx,
					body(t, rsp)))
			}

		})
	}
}

func TestServerNoContentType(t *testing.T) {
	apm, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer apm.Stop()

	req := makeTransactionRequest(t, apm.baseURL)
	rsp, err := apm.client.Do(req)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusBadRequest, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerSourcemapElasticsearch(t *testing.T) {
	for name, tc := range map[string]struct {
		expected     elasticsearch.Hosts
		config       m
		outputConfig m
	}{
		"nil": {
			expected: nil,
			config:   m{},
		},
		"esConfigured": {
			expected: elasticsearch.Hosts{"localhost:5200"},
			config: m{
				"rum": m{
					"enabled":                            "true",
					"source_mapping.elasticsearch.hosts": []string{"localhost:5200"},
				},
			},
		},
		"esFromOutput": {
			expected: elasticsearch.Hosts{"localhost:5201"},
			config: m{
				"rum": m{
					"enabled": "true",
				},
			},
			outputConfig: m{
				"elasticsearch": m{
					"enabled": true,
					"hosts":   []string{"localhost:5201"},
				},
			},
		},
		"esOutputDisabled": {
			expected: nil,
			config: m{
				"rum": m{
					"enabled": "true",
				},
			},
			outputConfig: m{
				"elasticsearch": m{
					"enabled": false,
					"hosts":   []string{"localhost:5202"},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ucfg, err := common.NewConfigFrom(tc.config)
			require.NoError(t, err)

			var beatConfig beat.BeatConfig
			ocfg, err := common.NewConfigFrom(tc.outputConfig)
			require.NoError(t, err)
			require.NoError(t, beatConfig.Output.Unpack(ocfg))

			apm, err := setupServer(t, ucfg, &beatConfig, nil)
			require.NoError(t, err)
			defer apm.Stop()

			if tc.expected != nil {
				assert.Equal(t, tc.expected, apm.config.RumConfig.SourceMapping.ESConfig.Hosts)
			}
		})
	}
}

func TestServerJaegerGRPC(t *testing.T) {
	server, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer server.Stop()

	baseURL, err := url.Parse(server.baseURL)
	require.NoError(t, err)
	conn, err := grpc.Dial(baseURL.Host, grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := api_v2.NewCollectorServiceClient(conn)
	result, err := client.PostSpans(context.Background(), &api_v2.PostSpansRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestServerOTLPGRPC(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"secret_token": "abc123"})
	assert.NoError(t, err)
	server, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer server.Stop()

	baseURL, err := url.Parse(server.baseURL)
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

	conn, err := grpc.Dial(baseURL.Host, grpc.WithInsecure())
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

func TestServerConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	// The beater has no way of unregistering itself from reload.Register,
	// so we create a fresh registry and replace it after the test.
	oldRegister := reload.Register
	defer func() {
		reload.Register = oldRegister
	}()
	reload.Register = reload.NewRegistry()

	cfg := common.MustNewConfigFrom(map[string]interface{}{
		// Set an invalid host to illustrate that the static config
		// is not used for defining the listening address.
		"host": "testing.invalid:123",

		// Data streams must be enabled when the server is managed.
		"data_streams.enabled": true,
	})
	apmBeat, cfg := newBeat(t, cfg, nil, nil)
	apmBeat.Manager = &mockManager{enabled: true}
	beater, err := newTestBeater(t, apmBeat, cfg, nil)
	require.NoError(t, err)
	beater.start()

	// Now that the beater is running, send config changes. The reloader
	// is not registered until after the beater starts running, so we
	// must loop until it is set.
	var reloadable reload.ReloadableList
	for {
		// The Reloader is not registered until after the beat has started running.
		reloadable = reload.Register.GetReloadableList("inputs")
		if reloadable != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The config must contain an "apm-server" section, and will be rejected otherwise.
	err = reloadable.Reload([]*reload.ConfigWithMeta{{Config: common.NewConfig()}})
	assert.EqualError(t, err, "'apm-server' not found in integration config")

	// Creating the socket listener is performed synchronously in the Reload method
	// to ensure zero downtime when reloading an already running server. Illustrate
	// that the socket listener is created synhconously in Reload by attempting to
	// reload with an invalid host.
	err = reloadable.Reload([]*reload.ConfigWithMeta{{Config: common.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"host": "testing.invalid:123",
		},
	})}})
	require.Error(t, err)
	assert.Regexp(t, "listen tcp: lookup testing.invalid: .*", err.Error())

	inputConfig := common.MustNewConfigFrom(map[string]interface{}{
		"apm-server": map[string]interface{}{
			"host": "localhost:0",
		},
	})
	err = reloadable.Reload([]*reload.ConfigWithMeta{{Config: inputConfig}})
	require.NoError(t, err)

	healthcheck := func(addr string) string {
		resp, err := http.Get("http://" + addr)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	addr1, err := beater.waitListenAddr(1 * time.Second)
	require.NoError(t, err)
	assert.NotEmpty(t, healthcheck(addr1)) // non-empty as there's no auth required

	// Reload config, causing the HTTP server to be restarted.
	require.NoError(t, inputConfig.SetString("apm-server.secret_token", -1, "secret"))
	err = reloadable.Reload([]*reload.ConfigWithMeta{{Config: inputConfig}})
	require.NoError(t, err)

	addr2, err := beater.waitListenAddr(1 * time.Second)
	require.NoError(t, err)
	assert.Empty(t, healthcheck(addr2)) // empty as auth is required but not specified

	// First HTTP server should have been stopped.
	_, err = http.Get("http://" + addr1)
	assert.Error(t, err)
}

func TestServerWaitForIntegrationKibana(t *testing.T) {
	var requests int
	requestCh := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":{"number":"1.2.3"}}`))
	})
	mux.HandleFunc("/api/fleet/epm/packages/apm", func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
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
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := common.MustNewConfigFrom(map[string]interface{}{
		"data_streams.enabled": true,
		"wait_ready_interval":  "100ms",
		"kibana.enabled":       true,
		"kibana.host":          srv.URL,
	})
	_, err := setupServer(t, cfg, nil, nil)
	require.NoError(t, err)

	timeout := time.After(10 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-requestCh:
		case <-timeout:
			t.Fatal("timed out waiting for request")
		}
	}
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
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := common.MustNewConfigFrom(map[string]interface{}{
		"data_streams.enabled": true,
		"wait_ready_interval":  "100ms",
	})
	var beatConfig beat.BeatConfig
	err := beatConfig.Output.Unpack(common.MustNewConfigFrom(map[string]interface{}{
		"elasticsearch": map[string]interface{}{
			"hosts":       []string{srv.URL},
			"backoff":     map[string]interface{}{"init": "10ms", "max": "10ms"},
			"max_retries": 1000,
		},
	}))
	require.NoError(t, err)

	beater, err := setupServer(t, cfg, &beatConfig, nil)
	require.NoError(t, err)

	// Send some events to the server. They should be accepted and enqueued.
	req := makeTransactionRequest(t, beater.baseURL)
	req.Header.Add("Content-Type", "application/x-ndjson")
	resp, err := beater.client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	resp.Body.Close()

	// Healthcheck should report that the server is not publish-ready.
	resp, err = beater.client.Get(beater.baseURL + api.RootPath)
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

	// Healthcheck should now report that the server is publish-ready.
	resp, err = beater.client.Get(beater.baseURL + api.RootPath)
	require.NoError(t, err)
	out = decodeJSONMap(t, resp.Body)
	resp.Body.Close()
	assert.Equal(t, true, out["publish_ready"])
}

type chanClient struct {
	done    chan struct{}
	Channel chan beat.Event
}

func newChanClientWith(ch chan beat.Event) *chanClient {
	if ch == nil {
		ch = make(chan beat.Event, 1)
	}
	c := &chanClient{
		done:    make(chan struct{}),
		Channel: ch,
	}
	return c
}

func (c *chanClient) Close() error {
	close(c.done)
	return nil
}

// Publish will publish every event in the batch on the channel. Options will be ignored.
// Always returns without error.
func (c *chanClient) Publish(_ context.Context, batch pubs.Batch) error {
	for _, event := range batch.Events() {
		select {
		case <-c.done:
		case c.Channel <- event.Content:
		}
	}
	batch.ACK()
	return nil
}

func (c *chanClient) String() string {
	return "event capturing test client"
}

type dummyOutputClient struct {
}

func (d *dummyOutputClient) Publish(_ context.Context, batch pubs.Batch) error {
	batch.ACK()
	return nil
}
func (d *dummyOutputClient) Close() error   { return nil }
func (d *dummyOutputClient) String() string { return "" }

func dummyPipeline(cfg *common.Config, info beat.Info, clients ...outputs.Client) *pipeline.Pipeline {
	if len(clients) == 0 {
		clients = []outputs.Client{&dummyOutputClient{}}
	}
	if cfg == nil {
		cfg = common.NewConfig()
	}
	processors, err := processing.MakeDefaultSupport(false)(info, logp.NewLogger("testbeat"), cfg)
	if err != nil {
		panic(err)
	}
	p, err := pipeline.New(
		info,
		pipeline.Monitors{},
		func(lis queue.ACKListener) (queue.Queue, error) {
			return memqueue.NewQueue(nil, memqueue.Settings{
				ACKListener: lis,
				Events:      20,
			}), nil
		},
		outputs.Group{
			Clients:   clients,
			BatchSize: 5,
			Retry:     0, // no retry. on error drop events
		},
		pipeline.Settings{
			WaitClose:     0,
			WaitCloseMode: pipeline.NoWaitOnClose,
			Processors:    processors,
		},
	)
	if err != nil {
		panic(err)
	}
	return p
}

var testData = func() []byte {
	b, err := ioutil.ReadFile("../testdata/intake-v2/transactions.ndjson")
	if err != nil {
		panic(err)
	}
	return b
}()

func makeTransactionRequest(t *testing.T, baseUrl string) *http.Request {
	req, err := http.NewRequest(http.MethodPost, baseUrl+api.IntakePath, bytes.NewReader(testData))
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
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return string(body)
}

type mockManager struct {
	management.Manager
	enabled bool
}

func (m *mockManager) Enabled() bool {
	return m.enabled
}
