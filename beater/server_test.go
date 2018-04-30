package beater

import (
	"bytes"
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

	"github.com/kabukky/httpscerts"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/common"
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
	apm, teardown := setupServer(t, nil)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, res.StatusCode, body(t, res))
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
	btr, stop := setupServer(t, ucfg)
	defer stop()

	baseUrl, client := btr.client(false)
	rsp, err := client.Get(baseUrl + HealthCheckURL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
}

func TestServerHealth(t *testing.T) {
	apm, teardown := setupServer(t, nil)
	defer teardown()

	baseUrl, client := apm.client(false)
	req, err := http.NewRequest("GET", baseUrl+HealthCheckURL, nil)
	assert.NoError(t, err)
	res, err := client.Do(req)
	assert.Equal(t, http.StatusOK, res.StatusCode, body(t, res))
}

func TestServerFrontendSwitch(t *testing.T) {
	ucfg, err := common.NewConfigFrom(m{"frontend": m{"enabled": true, "allow_origins": []string{"*"}}})
	assert.NoError(t, err)
	apm, teardown := setupServer(t, ucfg)
	defer teardown()

	baseUrl, client := apm.client(false)
	req, err := http.NewRequest("POST", baseUrl+FrontendTransactionsURL, bytes.NewReader(testData))
	assert.NoError(t, err)
	res, err := client.Do(req)
	assert.NotEqual(t, http.StatusForbidden, res.StatusCode, body(t, res))
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
		ucfg, err := common.NewConfigFrom(m{"frontend": m{"enabled": true, "allow_origins": test.allowedOrigins}})
		assert.NoError(t, err)
		var apm *beater
		apm, teardown = setupServer(t, ucfg)
		baseUrl, client := apm.client(false)
		req, err := http.NewRequest("POST", baseUrl+FrontendTransactionsURL, bytes.NewReader(testData))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Content-Type", "application/json")
		assert.NoError(t, err)
		res, err := client.Do(req)
		assert.Equal(t, test.expectedStatus, res.StatusCode, fmt.Sprintf("Failed at idx %v; %s", idx, body(t, res)))
		teardown()
	}
}

func TestServerNoContentType(t *testing.T) {
	apm, teardown := setupServer(t, nil)
	defer teardown()

	baseUrl, client := apm.client(false)
	req := makeTransactionRequest(t, baseUrl)
	res, error := client.Do(req)
	assert.NoError(t, error)
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, body(t, res))
}

func TestServerSSL(t *testing.T) {
	tests := []struct {
		label            string
		domain           string
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
			domain: "localhost", expectedMsgs: []string{"malformed HTTP response"}, overrideProtocol: true,
		},
	}
	var teardown = func() {}
	defer teardown() // in case test crashes. calling teardown twice is ok
	for idx, test := range tests {
		var apm *beater
		apm, teardown = setupServer(t, withSSL(t, test.domain))
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

func TestServerTcpConnLimit(t *testing.T) {
	t.Skip("tcp conn limit test disabled")

	if testing.Short() {
		t.Skip("skipping tcp conn limit test")
	}

	// this might make this test flaky, we'll see
	backlog := 128 // default net.core.somaxconn / kern.ipc.somaxconn
	maxConns := 1
	ucfg, err := common.NewConfigFrom(map[string]interface{}{
		"max_connections": maxConns,
	})
	assert.NoError(t, err)
	apm, teardown := setupServer(t, ucfg)
	defer teardown()

	conns := make([]net.Conn, backlog+maxConns)
	defer func() {
		for _, conn := range conns {
			if conn != nil {
				conn.Close()
			}
		}
	}()

	connect := func() (net.Conn, error) { return net.DialTimeout("tcp", apm.server.Addr, time.Second) }
	for i := 0; i < backlog+maxConns-1; i++ {
		conns[i], err = connect()
		if err != nil {
			t.Fatal(err)
		}
	}

	// ensure this is hit reasonably close to max conns, say within 150 conns
	// on some systems it's at connection 129, others at 131, still others 250
	for i := 0; i < 150; i++ {
		if _, err = connect(); err != nil {
			break
		}
	}
	if err == nil {
		t.Error("expected to reach tcp connection limit")
	}
}

func setupServer(t *testing.T, cfg *common.Config) (*beater, func()) {
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
	btr, stop := setupBeater(t, baseConfig)

	assert.NotEqual(t, btr.config.Host, "localhost:0", "config.Host unmodified")
	return btr, stop
}

var testData = func() []byte {
	d, err := loader.LoadValidDataAsBytes("transaction")
	if err != nil {
		panic(err)
	}
	return d
}()

func withSSL(t *testing.T, domain string) *common.Config {
	cert := path.Join(tmpCertPath, t.Name()+".crt")
	key := path.Join(tmpCertPath, t.Name()+".key")
	t.Log("generating certificate in", cert)
	httpscerts.Generate(cert, key, domain)

	cfg, err := common.NewConfigFrom(map[string]map[string]interface{}{
		"ssl": {
			"certificate": cert,
			"key":         key,
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

func waitForServer(url string, client *http.Client) {
	var check = func() int {
		var res *http.Response
		var err error
		res, err = client.Get(url + HealthCheckURL)
		if err != nil {
			return http.StatusInternalServerError
		}
		return res.StatusCode
	}

	for i := 0; i <= 1000; i++ {
		time.Sleep(time.Second / 50)
		if check() == http.StatusOK {
			return
		}
	}
	panic("server run timeout (10 seconds)")
}

func body(t *testing.T, response *http.Response) string {
	body, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err)
	return string(body)
}

func nopReporter(_ pendingReq) error { return nil }
