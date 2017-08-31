package beater

import (
	"bytes"
	"crypto/tls"
	"errors"
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

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
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

func TestDecode(t *testing.T) {
	transactionBytes, err := tests.LoadValidData("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)

	req, err := http.NewRequest("POST", "_", buffer)
	req.Header.Add("Content-Type", "application/json")
	assert.Nil(t, err)

	res, err := decodeData(req)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	body, err := ioutil.ReadAll(res)
	assert.Nil(t, err)
	assert.Equal(t, transactionBytes, body)
}

func TestServerOk(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	req := makeTestRequest(t)
	req.Header.Add("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rr, req)
	assert.Equal(t, 202, rr.Code, rr.Body.String())
}

func TestServerNoContentType(t *testing.T) {
	apm, teardown := setupServer(t, noSSL)
	defer teardown()

	rr := httptest.NewRecorder()
	apm.Handler.ServeHTTP(rr, makeTestRequest(t))
	assert.Equal(t, 400, rr.Code, rr.Body.String())
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
	assert.Equal(t, res.StatusCode, 202)
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

func TestSSLEnabled(t *testing.T) {
	truthy := true
	falsy := false

	cases := []struct {
		config   *SSLConfig
		expected bool
	}{
		{nil, false},
		{&SSLConfig{Enabled: &truthy}, true},
		{&SSLConfig{Enabled: &falsy}, false},
		{&SSLConfig{Cert: "Cert"}, true},
		{&SSLConfig{Cert: "Cert", PrivateKey: "key"}, true},
		{&SSLConfig{Cert: "Cert", PrivateKey: "key", Enabled: &falsy}, false},
	}

	for i, test := range cases {
		name := fmt.Sprintf("%v %v->%v", i, test.config, test.expected)
		t.Run(name, func(t *testing.T) {
			b := test.expected
			isEnabled := test.config.isEnabled()
			assert.Equal(t, b, isEnabled, "ssl config but should be %v", b)
		})
	}
}

func TestJSONFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)

	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
}

func TestJSONFailureResponseWhenAcceptingAnything(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`{"error":"Cannot compare apples to oranges"}`))
}

func TestHTMLFailureResponse(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
}

func TestFailureResponseNoAcceptHeader(t *testing.T) {
	req, err := http.NewRequest("POST", "/transactions", nil)
	assert.Nil(t, err)

	req.Header.Del("Accept")

	w := httptest.NewRecorder()
	sendStatus(w, req, 400, errors.New("Cannot compare apples to oranges"))

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, 400, w.Code)
	assert.Equal(t, body, []byte(`Cannot compare apples to oranges`))
}

func setupServer(t *testing.T, ssl *SSLConfig) (*http.Server, func()) {
	if testing.Short() {
		t.Skip("skipping server test")
	}

	host := randomAddr()
	cfg := defaultConfig
	cfg.Host = host
	cfg.SSL = ssl

	apm := newServer(cfg, nopReporter)
	go run(apm, cfg.SSL)

	secure := cfg.SSL != nil
	waitForServer(secure, host)

	return apm, func() { stop(apm, time.Second) }
}

var noSSL *SSLConfig

var testData []byte = func() []byte {
	d, err := tests.LoadValidData("transaction")
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
	req, err := http.NewRequest("POST", transaction.Endpoint, bytes.NewReader(testData))
	if err != nil {
		t.Fatalf("Failed to create test request object: %v", err)
	}

	return req
}

func postTestRequest(t *testing.T, apm *http.Server, client *http.Client, schema string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	addr := fmt.Sprintf("%s://%s%s", schema, apm.Addr, transaction.Endpoint)
	return client.Post(addr, "application/json", bytes.NewReader(testData))
}

func randomAddr() string {
	l, _ := net.Listen("tcp", "localhost:0")
	l.Close()
	return l.Addr().String()
}

func insecureClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

func waitForServer(secure bool, host string) {
	var check = func() int {
		var res *http.Response
		var err error
		if secure {
			res, err = insecureClient().Get("https://" + host + "/healthcheck")
		} else {
			res, err = http.Get("http://" + host + "/healthcheck")
		}

		if err != nil {
			return 500
		}
		return res.StatusCode
	}

	for i := 0; i <= 1000; i++ {
		time.Sleep(time.Second / 50)
		if check() == 200 {
			return
		}
	}
	panic("server run timeout (10 seconds)")
}

func nopReporter(_ []beat.Event) error { return nil }
