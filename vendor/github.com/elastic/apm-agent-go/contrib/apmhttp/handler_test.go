package apmhttp_test

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/elastic/apm-agent-go/contrib/apmhttp"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestWrap(t *testing.T) {
	mux := http.DefaultServeMux
	h := apmhttp.Wrap(mux)
	r := apmhttp.NewTraceRecovery(nil)
	h2 := h.WithRecovery(r)

	assert.Equal(t, &apmhttp.Handler{Handler: mux}, h)
	assert.NotEqual(t, h, h2)
	assert.NotNil(t, h2.Recovery)
	h2.Recovery = nil
	assert.Equal(t, h, h2)
}

func TestHandler(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	mux := http.NewServeMux()
	mux.Handle("/foo", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("bar"))
	}))

	h := &apmhttp.Handler{
		Handler: mux,
		Tracer:  tracer,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://server.testing/foo", nil)
	req.Header.Set("User-Agent", "apmhttp_test")
	req.RemoteAddr = "client.testing:1234"
	h.ServeHTTP(w, req)
	tracer.Flush(nil)

	payloads := transport.Payloads()
	transactions := payloads[0].Transactions()
	transaction := transactions[0]
	assert.Equal(t, "GET /foo", transaction.Name)
	assert.Equal(t, "request", transaction.Type)
	assert.Equal(t, "418", transaction.Result)

	true_ := true
	assert.Equal(t, &model.Context{
		Request: &model.Request{
			Socket: &model.RequestSocket{
				RemoteAddress: "client.testing",
			},
			URL: model.URL{
				Full:     "http://server.testing/foo",
				Protocol: "http",
				Hostname: "server.testing",
				Path:     "/foo",
			},
			Method: "GET",
			Headers: &model.RequestHeaders{
				UserAgent: "apmhttp_test",
			},
			HTTPVersion: "1.1",
		},
		Response: &model.Response{
			StatusCode: 418,
			Finished:   &true_,
		},
	}, transaction.Context)
}

func TestHandlerHTTP2(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	mux := http.NewServeMux()
	mux.Handle("/foo", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("bar"))
	}))
	srv := httptest.NewUnstartedServer(&apmhttp.Handler{
		Handler: mux,
		Tracer:  tracer,
	})
	err := http2.ConfigureServer(srv.Config, nil)
	require.NoError(t, err)
	srv.TLS = srv.Config.TLSConfig
	srv.StartTLS()
	defer srv.Close()
	srvAddr := srv.Listener.Addr().(*net.TCPAddr)

	client := &http.Client{Transport: &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}}
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", srv.URL+"/foo", nil)
	req.Header.Set("X-Real-IP", "client.testing")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	tracer.Flush(nil)

	payloads := transport.Payloads()
	transaction := payloads[0].Transactions()[0]

	true_ := true
	assert.Equal(t, &model.Context{
		Request: &model.Request{
			Socket: &model.RequestSocket{
				Encrypted:     true,
				RemoteAddress: "client.testing",
			},
			URL: model.URL{
				Full:     srv.URL + "/foo",
				Protocol: "https",
				Hostname: srvAddr.IP.String(),
				Port:     strconv.Itoa(srvAddr.Port),
				Path:     "/foo",
			},
			Method: "GET",
			Headers: &model.RequestHeaders{
				UserAgent: "Go-http-client/2.0",
			},
			HTTPVersion: "2.0",
		},
		Response: &model.Response{
			StatusCode: 418,
			Finished:   &true_,
		},
	}, transaction.Context)
}

func TestHandlerRecovery(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	h := &apmhttp.Handler{
		Handler:  http.HandlerFunc(panicHandler),
		Recovery: apmhttp.NewTraceRecovery(tracer),
		Tracer:   tracer,
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://server.testing/foo", nil)
	h.ServeHTTP(w, req)
	tracer.Flush(nil)

	payloads := transport.Payloads()
	error0 := payloads[0].Errors()[0]
	transaction := payloads[1].Transactions()[0]

	assert.Equal(t, "panicHandler", error0.Culprit)
	assert.Equal(t, "foo", error0.Exception.Message)

	true_ := true
	assert.Equal(t, &model.Response{
		Finished:   &true_,
		StatusCode: 418,
	}, transaction.Context.Response)
}

func panicHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusTeapot)
	panic("foo")
}
