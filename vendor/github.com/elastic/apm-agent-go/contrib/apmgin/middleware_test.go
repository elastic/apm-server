package apmgin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/contrib/apmgin"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func init() {
	// Make gin be quiet.
	gin.SetMode(gin.ReleaseMode)
}

func TestMiddleware(t *testing.T) {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	r := gin.New()
	r.Use(apmgin.Middleware(r, tracer))
	r.GET("/hello/:name", handleHello)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://server.testing/hello/isbel", nil)
	req.Header.Set("User-Agent", "apmgin_test")
	req.RemoteAddr = "client.testing:1234"
	r.ServeHTTP(w, req)
	tracer.Flush(nil)

	payloads := transport.Payloads()
	transactions := payloads[0].Transactions()
	transaction := transactions[0]
	assert.Equal(t, "GET /hello/:name", transaction.Name)
	assert.Equal(t, "request", transaction.Type)
	assert.Equal(t, "200", transaction.Result)

	true_ := true
	assert.Equal(t, &model.Context{
		Custom: model.IfaceMap{{
			"gin", map[string]interface{}{
				"handler": "github.com/elastic/apm-agent-go/contrib/apmgin_test.handleHello",
			},
		}},
		Request: &model.Request{
			Socket: &model.RequestSocket{
				RemoteAddress: "client.testing",
			},
			URL: model.URL{
				Full:     "http://server.testing/hello/isbel",
				Protocol: "http",
				Hostname: "server.testing",
				Path:     "/hello/isbel",
			},
			Method: "GET",
			Headers: &model.RequestHeaders{
				UserAgent: "apmgin_test",
			},
			HTTPVersion: "1.1",
		},
		Response: &model.Response{
			StatusCode:  200,
			Finished:    &true_,
			HeadersSent: &true_,
			Headers: &model.ResponseHeaders{
				ContentType: "text/plain; charset=utf-8",
			},
		},
	}, transaction.Context)
}
