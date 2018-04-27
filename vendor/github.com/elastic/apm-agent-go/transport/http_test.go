package transport_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport"
)

func init() {
	// Don't let the environment influence tests.
	os.Setenv("ELASTIC_APM_SERVER_URL", "")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "")
	os.Setenv("ELASTIC_APM_VERIFY_SERVER_CERT", "")
}

func TestNewHTTPTransportNoURL(t *testing.T) {
	_, err := transport.NewHTTPTransport("", "")
	assert.Error(t, err)
}

func TestHTTPTransportEnvURL(t *testing.T) {
	var h recordingHandler
	server := httptest.NewServer(&h)
	defer server.Close()

	defer patchEnv("ELASTIC_APM_SERVER_URL", server.URL)()

	transport, err := transport.NewHTTPTransport("", "")
	assert.NoError(t, err)
	transport.SendTransactions(context.Background(), &model.TransactionsPayload{})
	assert.Len(t, h.requests, 1)
}

func TestHTTPTransportSecretToken(t *testing.T) {
	var h recordingHandler
	server := httptest.NewServer(&h)
	defer server.Close()

	transport, err := transport.NewHTTPTransport(server.URL, "hunter2")
	assert.NoError(t, err)
	transport.SendTransactions(context.Background(), &model.TransactionsPayload{})

	assert.Len(t, h.requests, 1)
	assertAuthorization(t, h.requests[0], "hunter2")
}

func TestHTTPTransportEnvSecretToken(t *testing.T) {
	var h recordingHandler
	server := httptest.NewServer(&h)
	defer server.Close()

	defer patchEnv("ELASTIC_APM_SECRET_TOKEN", "hunter2")()

	transport, err := transport.NewHTTPTransport(server.URL, "")
	assert.NoError(t, err)
	transport.SendTransactions(context.Background(), &model.TransactionsPayload{})

	assert.Len(t, h.requests, 1)
	assertAuthorization(t, h.requests[0], "hunter2")
}

func TestHTTPTransportNoSecretToken(t *testing.T) {
	var h recordingHandler
	transport, server := newHTTPTransport(t, &h)
	defer server.Close()

	transport.SendTransactions(context.Background(), &model.TransactionsPayload{})

	assert.Len(t, h.requests, 1)
	assertAuthorization(t, h.requests[0], "")
}

func TestHTTPTransportTLS(t *testing.T) {
	var h recordingHandler
	server := httptest.NewUnstartedServer(&h)
	server.Config.ErrorLog = log.New(ioutil.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	transport, err := transport.NewHTTPTransport(server.URL, "")
	assert.NoError(t, err)

	p := &model.TransactionsPayload{}

	// Send should fail, because we haven't told the client
	// about the CA certificate, nor configured it to disable
	// certificate verification.
	err = transport.SendTransactions(context.Background(), p)
	assert.Error(t, err)

	// Reconfigure the transport so that it knows about the
	// CA certificate. We avoid using server.Client here, as
	// it is not available in older versions of Go.
	certificate, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	assert.NoError(t, err)
	certpool := x509.NewCertPool()
	certpool.AddCert(certificate)
	transport.Client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certpool,
		},
	}
	err = transport.SendTransactions(context.Background(), p)
	assert.NoError(t, err)
}

func TestHTTPTransportEnvVerifyServerCert(t *testing.T) {
	var h recordingHandler
	server := httptest.NewTLSServer(&h)
	defer server.Close()

	defer patchEnv("ELASTIC_APM_VERIFY_SERVER_CERT", "false")()

	transport, err := transport.NewHTTPTransport(server.URL, "")
	assert.NoError(t, err)

	assert.NotNil(t, transport.Client)
	assert.IsType(t, &http.Transport{}, transport.Client.Transport)
	httpTransport := transport.Client.Transport.(*http.Transport)
	assert.NotNil(t, httpTransport.TLSClientConfig)
	assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify)

	err = transport.SendTransactions(context.Background(), &model.TransactionsPayload{})
	assert.NoError(t, err)
}

func TestHTTPError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "error-message", http.StatusInternalServerError)
	})
	tr, server := newHTTPTransport(t, h)
	defer server.Close()

	err := tr.SendTransactions(context.Background(), &model.TransactionsPayload{})
	assert.EqualError(t, err, "SendTransactions failed with 500 Internal Server Error: error-message")
}

func TestConcurrentSendTransactions(t *testing.T) {
	payload := &model.TransactionsPayload{
		Service: &model.Service{},
	}
	testConcurrentSend(t, func(tr transport.Transport) {
		tr.SendTransactions(context.Background(), payload)
	})
}

func TestConcurrentSendErrors(t *testing.T) {
	payload := &model.ErrorsPayload{
		Service: &model.Service{},
	}
	testConcurrentSend(t, func(tr transport.Transport) {
		tr.SendErrors(context.Background(), payload)
	})
}

func testConcurrentSend(t *testing.T, write func(transport.Transport)) {
	transport, server := newHTTPTransport(t, nopHandler{})
	defer server.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				write(transport)
			}
		}()
	}
	wg.Wait()
}

func newHTTPTransport(t *testing.T, handler http.Handler) (*transport.HTTPTransport, *httptest.Server) {
	server := httptest.NewServer(handler)
	transport, err := transport.NewHTTPTransport(server.URL, "")
	if !assert.NoError(t, err) {
		server.Close()
		t.FailNow()
	}
	return transport, server
}

type nopHandler struct{}

func (nopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

type recordingHandler struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.requests = append(h.requests, req)
}

func assertAuthorization(t *testing.T, req *http.Request, token string) {
	values, ok := req.Header["Authorization"]
	if !ok {
		if token == "" {
			return
		}
		t.Errorf("missing Authorization header")
		return
	}
	var expect []string
	if token != "" {
		expect = []string{"Bearer " + token}
	}
	assert.Equal(t, expect, values)
}

func patchEnv(key, value string) func() {
	old, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		panic(err)
	}
	return func() {
		var err error
		if !had {
			err = os.Unsetenv(key)
		} else {
			err = os.Setenv(key, old)
		}
		if err != nil {
			panic(err)
		}
	}
}
