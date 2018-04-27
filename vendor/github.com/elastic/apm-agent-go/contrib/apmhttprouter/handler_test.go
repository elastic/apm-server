package apmhttprouter_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/contrib/apmhttp"
	"github.com/elastic/apm-agent-go/contrib/apmhttprouter"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestWrapHandle(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	router := httprouter.New()

	const route = "/hello/:name/go/*wild"
	router.GET(route, apmhttprouter.WrapHandle(
		func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte(fmt.Sprintf("%s:%s", p.ByName("name"), p.ByName("wild"))))
		},
		tracer, route,
		nil, // no recovery
	))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://server.testing/hello/go/go/bananas", nil)
	req.Header.Set("User-Agent", "apmhttp_test")
	req.RemoteAddr = "client.testing:1234"
	router.ServeHTTP(w, req)
	tracer.Flush(nil)
	assert.Equal(t, "go:/bananas", w.Body.String())

	payloads := transport.Payloads()
	transactions := payloads[0].Transactions()
	transaction := transactions[0]
	assert.Equal(t, "GET /hello/:name/go/*wild", transaction.Name)
	assert.Equal(t, "request", transaction.Type)
	assert.Equal(t, "418", transaction.Result)

	true_ := true
	assert.Equal(t, &model.Context{
		Request: &model.Request{
			Socket: &model.RequestSocket{
				RemoteAddress: "client.testing",
			},
			URL: model.URL{
				Full:     "http://server.testing/hello/go/go/bananas",
				Protocol: "http",
				Hostname: "server.testing",
				Path:     "/hello/go/go/bananas",
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

func TestRecovery(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	router := httprouter.New()

	const route = "/panic"
	router.GET(route, apmhttprouter.WrapHandle(
		panicHandler, tracer, route,
		apmhttp.NewTraceRecovery(tracer),
	))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://server.testing/panic", nil)
	router.ServeHTTP(w, req)
	tracer.Flush(nil)
	assert.Equal(t, http.StatusTeapot, w.Code)

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

func panicHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusTeapot)
	panic("foo")
}
