package beater

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kabukky/httpscerts"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
)

var tmpCertPath string

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
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	req := makeTestRequest(t)
	req.Header.Add("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusAccepted, rr.Code, rr.Body.String())
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
	addr := tmpTestUnix(t)
	ucfg, err := common.NewConfigFrom(map[string]interface{}{
		"host": "unix:" + addr,
	})
	assert.NoError(t, err)
	btr, stop := setupBeater(t, ucfg)
	defer stop()

	baseUrl, client := btr.client()
	rsp, err := client.Get(baseUrl + HealthCheckURL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
}

func TestServerHealth(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	req, err := http.NewRequest("GET", HealthCheckURL, nil)
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	rr := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, rr.Code)
}

func TestServerFrontendSwitch(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	req, _ := http.NewRequest("POST", FrontendTransactionsURL, bytes.NewReader(testData))

	rec := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rec, req)
	apm.Handler = newMuxer(
		&Config{
			Frontend: &FrontendConfig{Enabled: new(bool), AllowOrigins: []string{"*"}}},
		nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	true := true
	apm.Handler = newMuxer(
		&Config{
			Frontend: &FrontendConfig{Enabled: &true, AllowOrigins: []string{"*"}}},
		nil)
	rec = httptest.NewRecorder()
	apm.Handler.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestServerCORS(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

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

	for idx, test := range tests {
		apm.Handler = newMuxer(
			&Config{
				MaxUnzippedSize:     1024 * 1024,
				ConcurrentRequests:  40,
				MaxRequestQueueTime: time.Second * 10,
				Frontend: &FrontendConfig{
					Enabled:      &true,
					RateLimit:    10,
					AllowOrigins: test.allowedOrigins},
			},
			nopReporter)
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", FrontendTransactionsURL, bytes.NewReader(testData))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Content-Type", "application/json")
		apm.Handler.ServeHTTP(rec, req)

		assert.Equal(t, test.expectedStatus, rec.Code, fmt.Sprintf("Failed at idx %v; %s", idx, rec.Body.String()))
	}
}

func TestServerNoContentType(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	rr := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rr, makeTestRequest(t))
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestServerSecureUnknownCA(t *testing.T) {
	apm, teardown := setupServer(t, withSSL(t, "127.0.0.1"))
	defer teardown()

	_, err := postTestRequest(t, apm, nil, "https")
	assert.Contains(t, err.Error(), "x509: certificate signed by unknown authority")
}

func TestServerSecureSkipVerify(t *testing.T) {
	apm, teardown := setupServer(t, withSSL(t, "127.0.0.1"))
	defer teardown()

	res, err := postTestRequest(t, apm, insecureClient(), "https")
	assert.Nil(t, err)
	assert.Equal(t, res.StatusCode, http.StatusAccepted)
}

func TestServerSecureBadDomain(t *testing.T) {
	apm, teardown := setupServer(t, withSSL(t, "ELASTIC"))
	defer teardown()

	_, err := postTestRequest(t, apm, nil, "https")

	msgs := []string{
		"x509: certificate signed by unknown authority",
		"x509: cannot validate certificate for 127.0.0.1",
	}
	checkErrMsg := strings.Contains(err.Error(), msgs[0]) || strings.Contains(err.Error(), msgs[1])
	assert.True(t, checkErrMsg, err.Error())
}

func TestServerSecureBadIP(t *testing.T) {
	apm, teardown := setupServer(t, withSSL(t, "192.168.10.11"))
	defer teardown()

	_, err := postTestRequest(t, apm, nil, "https")
	msgs := []string{
		"x509: certificate signed by unknown authority",
		"x509: certificate is valid for 192.168.10.11, not 127.0.0.1",
	}
	checkErrMsg := strings.Contains(err.Error(), msgs[0]) || strings.Contains(err.Error(), msgs[1])
	assert.True(t, checkErrMsg, err.Error())
}

func TestServerBadProtocol(t *testing.T) {
	apm, teardown := setupServer(t, withSSL(t, "localhost"))
	defer teardown()

	_, err := postTestRequest(t, apm, nil, "http")
	assert.Contains(t, err.Error(), "malformed HTTP response")
}

func TestServerTcpConnLimit(t *testing.T) {
	t.Skip("tcp conn limit test disabled")

	if testing.Short() {
		t.Skip("skipping tcp conn limit test")
	}

	// this might make this test flaky, we'll see
	backlog := 128 // default net.core.somaxconn / kern.ipc.somaxconn
	maxConns := 1
	apm, teardown := setupServer(t, nil, func(c *Config) { c.MaxConnections = maxConns })
	defer teardown()

	conns := make([]net.Conn, backlog+maxConns)
	defer func() {
		for _, conn := range conns {
			if conn != nil {
				conn.Close()
			}
		}
	}()

	connect := func() (net.Conn, error) { return net.DialTimeout("tcp", apm.Addr, time.Second) }
	var err error
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

type configMutator func(*Config)

func setupServer(t *testing.T, ssl *SSLConfig, mutators ...configMutator) (*http.Server, func()) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	cfg := defaultConfig("7.0.0")
	cfg.Host = lis.Addr().String()
	cfg.SSL = ssl
	for _, m := range mutators {
		m(cfg)
	}

	apm := newServer(cfg, nopReporter)
	go run(apm, lis, cfg)

	client := &http.Client{}
	scheme := "http://"
	if cfg.SSL != nil {
		client = insecureClient()
		scheme = "https://"
	}
	waitForServer(scheme+cfg.Host, client)

	return apm, func() { stop(apm) }
}

var noSSL *SSLConfig

var testData = func() []byte {
	d, err := loader.LoadValidDataAsBytes("transaction")
	if err != nil {
		panic(err)
	}
	return d
}()

func withSSL(t *testing.T, domain string) *SSLConfig {
	cert := path.Join(tmpCertPath, t.Name()+".crt")
	key := path.Join(tmpCertPath, t.Name()+".key")
	t.Log("generating certificate in ", cert)
	httpscerts.Generate(cert, key, domain)

	return &SSLConfig{Cert: cert, PrivateKey: key}
}

func makeTestRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("POST", BackendTransactionsURL, bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	return req
}

func postTestRequest(t *testing.T, apm *http.Server, client *http.Client, schema string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	addr := fmt.Sprintf("%s://%s%s", schema, apm.Addr, BackendTransactionsURL)
	return client.Post(addr, "application/json", bytes.NewReader(testData))
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

func nopReporter(_ []beat.Event) error { return nil }
