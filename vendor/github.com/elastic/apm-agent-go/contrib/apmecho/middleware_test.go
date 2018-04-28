package apmecho_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go/contrib/apmecho"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestEchoMiddleware(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	e := echo.New()
	e.Use(apmecho.Middleware(tracer))
	e.GET("/hello/:name", handleHello)

	w := doRequest(e, "GET", "http://server.testing/hello/foo")
	assert.Equal(t, "Hello, foo!", w.Body.String())
	tracer.Flush(nil)

	payloads := transport.Payloads()
	transaction := payloads[0].Transactions()[0]

	assert.Equal(t, "GET /hello/:name", transaction.Name)
	assert.Equal(t, "request", transaction.Type)
	assert.Equal(t, "200", transaction.Result)

	true_ := true
	assert.Equal(t, &model.Context{
		Request: &model.Request{
			Socket: &model.RequestSocket{
				RemoteAddress: "client.testing",
			},
			URL: model.URL{
				Full:     "http://server.testing/hello/foo",
				Protocol: "http",
				Hostname: "server.testing",
				Path:     "/hello/foo",
			},
			Method:      "GET",
			HTTPVersion: "1.1",
			Headers: &model.RequestHeaders{
				UserAgent: "apmecho_test",
			},
		},
		Response: &model.Response{
			StatusCode:  200,
			HeadersSent: &true_,
			Headers: &model.ResponseHeaders{
				ContentType: "text/plain; charset=UTF-8",
			},
		},
	}, transaction.Context)
}

func TestEchoMiddlewarePanic(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	e := echo.New()
	e.Use(apmecho.Middleware(tracer))
	e.GET("/panic", handlePanic)

	w := doRequest(e, "GET", "http://server.testing/panic")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	tracer.Flush(nil)
	assertError(t, transport.Payloads(), "handlePanic", "boom", false)
}

func TestEchoMiddlewareError(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	e := echo.New()
	e.Use(apmecho.Middleware(tracer))
	e.GET("/error", handleError)

	w := doRequest(e, "GET", "http://server.testing/error")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	tracer.Flush(nil)
	assertError(t, transport.Payloads(), "handleError", "wot", true)
}

func assertError(t *testing.T, payloads transporttest.Payloads, culprit, message string, handled bool) {
	error0 := payloads[0].Errors()[0]

	require.NotNil(t, error0.Context)
	require.NotNil(t, error0.Exception)
	assert.NotEmpty(t, error0.Transaction.ID)
	assert.Equal(t, culprit, error0.Culprit)
	assert.Equal(t, message, error0.Exception.Message)
	assert.Equal(t, handled, error0.Exception.Handled)
}

func handleHello(c echo.Context) error {
	return c.String(http.StatusOK, fmt.Sprintf("Hello, %s!", c.Param("name")))
}

func handlePanic(c echo.Context) error {
	panic("boom")
}

func handleError(c echo.Context) error {
	return errors.New("wot")
}

func doRequest(e *echo.Echo, method, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("User-Agent", "apmecho_test")
	req.RemoteAddr = "client.testing:1234"
	e.ServeHTTP(w, req)
	return w
}
