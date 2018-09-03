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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/outputs/transport/transptest"
)

var tmpCertPath string

type m map[string]interface{}

func TestMain(m *testing.M) {
	current, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	tmpCertPath = filepath.Join(current, "test_certs")
	os.Mkdir(tmpCertPath, os.ModePerm)

	code := m.Run()
	if code == 0 {
		os.RemoveAll(tmpCertPath)
	}
	os.Exit(code)
}

func TestServerOk(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
}

func TestServerOkV2(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionV2Request(t, baseUrl)
	req.Header.Add("Content-Type", "application/x-ndjson")
	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
}

func TestServerRoot(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
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
			var rsp map[string]interface{}
			b, err := ioutil.ReadAll(res.Body)
			require.NoError(t, err)
			if err := json.Unmarshal(b, &rsp); err != nil {
				t.Fatal(err, b)
			}
			assert.Equal(t, hasOk, rsp["ok"] != nil, string(b))
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
		{path: "/foo", accept: &jsonContent, expectStatus: http.StatusNotFound, expectContentType: plain},
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
	ucfg, err := common.NewConfigFrom(map[string]interface{}{"secret_token": token})
	assert.NoError(t, err)
	apm, teardown, err := setupServer(t, ucfg, nil)
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
	// try to connect to localhost:defaultPort
	// if connection succeeds, port is in use and skip test
	// if it fails, make sure it is because connection refused
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", defaultPort), 2*time.Second); err == nil {
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
	btr, teardown, err := setupServer(t, ucfg, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := btr.client(false)
	rsp, err := client.Get(baseUrl + HealthCheckURL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
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
	ucfg, err := common.NewConfigFrom(m{"host": "unix:" + addr})
	assert.NoError(t, err)
	btr, stop, err := setupServer(t, ucfg, nil)
	require.NoError(t, err)
	defer stop()

	baseUrl, client := btr.client(false)
	rsp, err := client.Get(baseUrl + HealthCheckURL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
}

func TestServerHealth(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req, err := http.NewRequest("GET", baseUrl+HealthCheckURL, nil)
	assert.NoError(t, err)
	res, err := client.Do(req)
	assert.Equal(t, http.StatusOK, res.StatusCode, body(t, res))
}

func TestServerRumSwitch(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"rum": m{"enabled": true, "allow_origins": []string{"*"}}})
	assert.NoError(t, err)
	apm, teardown, err := setupServer(t, ucfg, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)

	for _, url := range []string{
		RumTransactionsURL,
		V2RumURL,
	} {
		req, err := http.NewRequest("POST", baseUrl+url, bytes.NewReader(testData))
		assert.NoError(t, err)
		res, err := client.Do(req)
		assert.NotEqual(t, http.StatusForbidden, res.StatusCode, body(t, res))
	}
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
		apm, teardown, err = setupServer(t, ucfg, nil)
		require.NoError(t, err)
		baseUrl, client := apm.client(false)

		for _, endpoint := range []struct {
			url, contentType string
			testData         []byte
		}{
			{RumTransactionsURL, "application/json", testData},
			{V2RumURL, "application/x-ndjson", testDataV2},
		} {
			req, err := http.NewRequest("POST", baseUrl+endpoint.url, bytes.NewReader(endpoint.testData))
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Content-Type", endpoint.contentType)
			assert.NoError(t, err)
			res, err := client.Do(req)
			assert.Equal(t, test.expectedStatus, res.StatusCode, fmt.Sprintf("Failed at idx %v; %s", idx, body(t, res)))
		}
		teardown()
	}
}

func TestServerNoContentType(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	res, error := client.Do(req)
	assert.NoError(t, error)
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, body(t, res))
}

func TestServerNoContentTypeV2(t *testing.T) {
	apm, teardown, err := setupServer(t, nil, nil)
	require.NoError(t, err)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionV2Request(t, baseUrl)
	res, error := client.Do(req)
	assert.NoError(t, error)
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, body(t, res))
}

func TestServerSourcemapElasticsearch(t *testing.T) {
	cases := []struct {
		expected     []string
		config       m
		outputConfig m
	}{
		{
			expected: nil,
			config:   m{},
		},
		{
			// source_mapping.elasticsearch.hosts set
			expected: []string{"localhost:5200"},
			config: m{
				"frontend": m{
					"enabled": "true",
					"source_mapping.elasticsearch.hosts": []string{"localhost:5200"},
				},
			},
		},
		{
			// source_mapping.elasticsearch.hosts not set, elasticsearch.enabled = true
			expected: []string{"localhost:5201"},
			config: m{
				"frontend": m{
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
		{
			// source_mapping.elasticsearch.hosts not set, elasticsearch.enabled = false
			expected: nil,
			config: m{
				"frontend": m{
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
	}
	for _, testCase := range cases {
		ucfg, err := common.NewConfigFrom(testCase.config)
		if !assert.NoError(t, err) {
			continue
		}

		var beatConfig beat.BeatConfig
		ocfg, err := common.NewConfigFrom(testCase.outputConfig)
		if !assert.NoError(t, err) {
			continue
		}
		beatConfig.Output.Unpack(ocfg)
		apm, teardown, err := setupServer(t, ucfg, &beatConfig)
		if assert.NoError(t, err) {
			assert.Equal(t, testCase.expected, apm.smapElasticsearchHosts())
		}
		teardown()
	}
}

func TestServerSSL(t *testing.T) {
	tests := []struct {
		label            string
		domain           string
		passphrase       string
		expectedMsgs     []string
		insecure         bool
		statusCode       int
		overrideProtocol bool
	}{
		{
			label: "unknown CA", domain: "127.0.0.1", expectedMsgs: []string{"x509: certificate signed by unknown authority"},
		},
		{
			label: "skip verification", domain: "127.0.0.1", insecure: true, statusCode: http.StatusAccepted,
		},
		{
			label:  "bad domain",
			domain: "ELASTIC", expectedMsgs: []string{
				"x509: certificate signed by unknown authority",
				"x509: cannot validate certificate for 127.0.0.1",
			},
		},
		{
			label:  "bad IP",
			domain: "192.168.10.11", expectedMsgs: []string{
				"x509: certificate signed by unknown authority",
				"x509: certificate is valid for 192.168.10.11, not 127.0.0.1",
			},
		},
		{
			label: "bad schema", domain: "localhost", expectedMsgs: []string{
				"malformed HTTP response",
				"transport connection broken"},
			overrideProtocol: true,
		},
		{
			label: "with passphrase", domain: "localhost", statusCode: http.StatusAccepted, insecure: true, passphrase: "foobar",
		},
	}
	var teardown = func() {}
	defer teardown() // in case test crashes. calling teardown twice is ok
	for idx, test := range tests {
		var apm *beater
		var err error
		apm, teardown, err = setupServer(t, withSSL(t, test.domain, test.passphrase), nil)
		require.NoError(t, err)
		baseUrl, client := apm.client(test.insecure)
		if test.overrideProtocol {
			baseUrl = strings.Replace(baseUrl, "https", "http", 1)
		}
		req := makeTransactionRequest(t, baseUrl)
		req.Header.Add("Content-Type", "application/json")
		res, err := client.Do(req)

		if len(test.expectedMsgs) > 0 {
			var containsErrMsg bool
			for _, msg := range test.expectedMsgs {
				containsErrMsg = containsErrMsg || strings.Contains(err.Error(), msg)
			}
			assert.True(t, containsErrMsg,
				fmt.Sprintf("expected %v at idx %d (%s)", err, idx, test.label))
		}

		if test.statusCode != 0 {
			assert.Equal(t, res.StatusCode, test.statusCode,
				fmt.Sprintf("wrong code at idx %d (%s)", idx, test.label))
		}
		teardown()
	}
}

func TestServerSecureBadPassphrase(t *testing.T) {
	withSSL(t, "127.0.0.1", "foo")
	name := path.Join(tmpCertPath, t.Name())
	cfg, err := common.NewConfigFrom(map[string]map[string]interface{}{
		"ssl": {
			"certificate":    name + ".pem",
			"key":            name + ".key",
			"key_passphrase": "bar",
		},
	})
	assert.NoError(t, err)
	_, _, err = setupServer(t, cfg, nil)
	if assert.Error(t, err) {
		b := strings.Contains(err.Error(), "no PEM blocks") ||
			strings.Contains(err.Error(), "failed to parse private key")
		assert.True(t, b, err.Error())
	}

}

func setupServer(t *testing.T, cfg *common.Config, beatConfig *beat.BeatConfig) (*beater, func(), error) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	baseConfig, err := common.NewConfigFrom(map[string]interface{}{
		"host": "localhost:0",
	})
	assert.NoError(t, err)
	if cfg != nil {
		err = cfg.Unpack(baseConfig)
	}
	assert.NoError(t, err)
	btr, stop, err := setupBeater(t, DummyPipeline(), baseConfig, beatConfig)
	if err == nil {
		assert.NotEqual(t, btr.config.Host, "localhost:0", "config.Host unmodified")
	}
	return btr, stop, err
}

var testData = func() []byte {
	d, err := loader.LoadValidDataAsBytes("transaction")
	if err != nil {
		panic(err)
	}
	return d
}()

var testDataV2 = func() []byte {
	b, err := loader.LoadDataAsBytes("../testdata/intake-v2/transactions.ndjson")
	if err != nil {
		panic(err)
	}
	return b
}()

func withSSL(t *testing.T, domain, passphrase string) *common.Config {
	name := path.Join(tmpCertPath, t.Name())
	t.Log("generating certificate in", name)
	err := transptest.GenCertForTestingPurpose(t, domain, name, passphrase)
	assert.NoError(t, err)
	cfg, err := common.NewConfigFrom(map[string]map[string]interface{}{
		"ssl": {
			"certificate":    name + ".pem",
			"key":            name + ".key",
			"key_passphrase": passphrase,
		},
	})
	assert.NoError(t, err)
	return cfg
}

func makeTransactionRequest(t *testing.T, baseUrl string) *http.Request {
	req, err := http.NewRequest("POST", baseUrl+BackendTransactionsURL, bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	return req
}

func makeTransactionV2Request(t *testing.T, baseUrl string) *http.Request {
	req, err := http.NewRequest("POST", baseUrl+V2BackendURL, bytes.NewReader(testDataV2))
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	return req
}

func waitForServer(url string, client *http.Client, c chan error) {
	var check = func() int {
		var res *http.Response
		var err error
		res, err = client.Get(url + HealthCheckURL)
		if err != nil {
			return http.StatusInternalServerError
		}
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
	assert.NoError(t, err)
	return string(body)
}

func nopReporter(context.Context, publish.PendingReq) error { return nil }
