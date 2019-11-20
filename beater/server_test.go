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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/gofrs/uuid"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	pubs "github.com/elastic/beats/libbeat/publisher"
	"github.com/elastic/beats/libbeat/publisher/pipeline"
	"github.com/elastic/beats/libbeat/publisher/processing"
	"github.com/elastic/beats/libbeat/publisher/queue"
	"github.com/elastic/beats/libbeat/publisher/queue/memqueue"
	"github.com/elastic/beats/libbeat/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/tests/loader"
)

type m map[string]interface{}

func TestServerOk(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	req.Header.Add("Content-Type", "application/x-ndjson")
	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
}

func TestServerRoot(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	rootRequest := func(path string, accept *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, baseUrl+path, nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if accept != nil {
			req.Header.Add("Accept", *accept)
		}
		res, err := client.Do(req)
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
	apm, teardown, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer teardown()
	baseUrl, client := apm.client(false)

	rootRequest := func(token *string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, baseUrl+"/", nil)
		require.NoError(t, err, "Failed to create test request object: %v", err)
		if token != nil {
			req.Header.Add("Authorization", "Bearer "+*token)
		}
		res, err := client.Do(req)
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
	btr, teardown, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := btr.client(false)
	rsp, err := client.Get(baseUrl + api.RootPath)
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
	btr, stop, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer stop()

	baseUrl, client := btr.client(false)
	rsp, err := client.Get(baseUrl + api.RootPath)
	assert.NoError(t, err)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerHealth(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req, err := http.NewRequest(http.MethodGet, baseUrl+api.RootPath, nil)
	require.NoError(t, err)
	rsp, err := client.Do(req)
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusOK, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerRumSwitch(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "allow_origins": []string{"*"}}})
	assert.NoError(t, err)
	apm, teardown, err := setupServer(t, ucfg, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)

	req, err := http.NewRequest(http.MethodPost, baseUrl+api.IntakeRUMPath, bytes.NewReader(testData))
	require.NoError(t, err)
	rsp, err := client.Do(req)
	if assert.NoError(t, err) {
		assert.NotEqual(t, http.StatusForbidden, rsp.StatusCode, body(t, rsp))
	}
}

func TestServerSourcemapBadConfig(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "source_mapping": m{"elasticsearch": m{"hosts": []string{}}}}})
	require.NoError(t, err)
	s, teardown, err := setupServer(t, ucfg, nil, nil)
	require.Nil(t, s)
	if err == nil {
		defer teardown()
	}
	require.Error(t, err)
}

func TestServerCORS(t *testing.T) {
	true := true
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

	var teardown = func() {}
	defer teardown() // in case test crashes. calling teardown twice is ok
	for idx, test := range tests {
		ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "allow_origins": test.allowedOrigins}})
		assert.NoError(t, err)
		var apm *beater
		apm, teardown, err = setupServer(t, ucfg, nil, nil)
		require.NoError(t, err)
		baseUrl, client := apm.client(false)

		req, err := http.NewRequest(http.MethodPost, baseUrl+api.IntakeRUMPath, bytes.NewReader(testData))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Content-Type", "application/x-ndjson")
		assert.NoError(t, err)
		rsp, err := client.Do(req)
		if assert.NoError(t, err) {
			assert.Equal(t, test.expectedStatus, rsp.StatusCode, fmt.Sprintf("Failed at idx %v; %s", idx,
				body(t, rsp)))
		}

		teardown()
	}
}

func TestServerNoContentType(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	rsp, err := client.Do(req)
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
			apm, teardown, err := setupServer(t, ucfg, &beatConfig, nil)
			require.NoError(t, err)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, apm.config.RumConfig.SourceMapping.ESConfig.Hosts)
			}
			teardown()
		})
	}
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
func (c *chanClient) Publish(batch pubs.Batch) error {
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

func (d *dummyOutputClient) Publish(batch pubs.Batch) error {
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
	processors, err := processing.MakeDefaultObserverSupport(false)(info, logp.NewLogger("testbeat"), cfg)
	if err != nil {
		panic(err)
	}
	p, err := pipeline.New(
		info,
		pipeline.Monitors{},
		func(e queue.Eventer) (queue.Queue, error) {
			return memqueue.NewBroker(nil, memqueue.Settings{
				Eventer: e,
				Events:  20,
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

func setupServer(t *testing.T, cfg *common.Config, beatConfig *beat.BeatConfig,
	events chan beat.Event) (*beater, func(), error) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	baseConfig := common.MustNewConfigFrom(map[string]interface{}{
		"host": "localhost:0",
	})
	if cfg != nil {
		err := cfg.Unpack(baseConfig)
		require.NoError(t, err)
	}

	beatId, err := uuid.FromString("fbba762a-14dd-412c-b7e9-b79f903eb492")
	require.NoError(t, err)
	info := beat.Info{
		Beat:        "test-apm-server",
		IndexPrefix: "test-apm-server",
		Version:     version.GetDefaultVersion(),
		ID:          beatId,
	}

	var pub beat.Pipeline
	if events != nil {
		// capture events
		pubClient := newChanClientWith(events)
		pub = dummyPipeline(cfg, info, pubClient)
	} else {
		// don't capture events
		pub = dummyPipeline(cfg, info)
	}

	// create a beat
	apmBeat := &beat.Beat{
		Publisher: pub,
		Info:      info,
		Config:    beatConfig,
	}

	btr, stop, err := setupBeater(t, apmBeat, baseConfig, beatConfig)
	if err == nil {
		assert.NotEqual(t, btr.config.Host, "localhost:0", "config.Host unmodified")
	}
	return btr, stop, err
}

var testData = func() []byte {
	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
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

func waitForServer(url string, client *http.Client, c chan error) {
	var check = func() int {
		var res *http.Response
		var err error
		res, err = client.Get(url + api.RootPath)
		if err != nil {
			return http.StatusInternalServerError
		}
		res.Body.Close()
		return res.StatusCode
	}

	for {
		time.Sleep(time.Second / 50)
		if check() == http.StatusOK {
			c <- nil
		}
	}
}

func body(t *testing.T, response *http.Response) string {
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return string(body)
}
