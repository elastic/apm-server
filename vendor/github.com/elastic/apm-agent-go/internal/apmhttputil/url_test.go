package apmhttputil_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/internal/apmhttputil"
	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

func TestRequestURLClient(t *testing.T) {
	req := mustNewRequest("https://user:pass@host.invalid:9443/path?query&querier=foo#fragment")
	assert.Equal(t, model.URL{
		Protocol: "https",
		Hostname: "host.invalid",
		Port:     "9443",
		Path:     "/path",
		Search:   "query&querier=foo",
		Hash:     "fragment",
	}, apmhttputil.RequestURL(req, nil))
}

func TestRequestURLServer(t *testing.T) {
	req := mustNewRequest("/path?query&querier=foo")
	req.Host = "host.invalid:8080"

	assert.Equal(t, model.URL{
		Protocol: "http",
		Hostname: "host.invalid",
		Port:     "8080",
		Path:     "/path",
		Search:   "query&querier=foo",
	}, apmhttputil.RequestURL(req, nil))
}

func TestRequestURLServerTLS(t *testing.T) {
	req := mustNewRequest("/path?query&querier=foo")
	req.Host = "host.invalid:8080"
	req.TLS = &tls.ConnectionState{}
	assert.Equal(t, "https", apmhttputil.RequestURL(req, nil).Protocol)
}

func TestRequestURLHeaders(t *testing.T) {
	type test struct {
		name      string
		full      string
		header    http.Header
		forwarded *apmhttputil.ForwardedHeader
	}

	tests := []test{{
		name:      "Forwarded",
		full:      "https://forwarded.invalid:443/",
		forwarded: &apmhttputil.ForwardedHeader{Host: "forwarded.invalid:443", Proto: "HTTPS"},
	}, {
		name:      "Forwarded-Empty-Host",
		full:      "http://host.invalid/", // falls back to the next option
		forwarded: &apmhttputil.ForwardedHeader{Host: ""},
	}, {
		name:   "X-Forwarded-Host",
		full:   "http://x-forwarded-host.invalid/",
		header: http.Header{"X-Forwarded-Host": []string{"x-forwarded-host.invalid"}},
	}, {
		name:   "X-Forwarded-Proto",
		full:   "https://host.invalid/",
		header: http.Header{"X-Forwarded-Proto": []string{"https"}},
	}, {
		name:   "X-Forwarded-Protocol",
		full:   "https://host.invalid/",
		header: http.Header{"X-Forwarded-Protocol": []string{"https"}},
	}, {
		name:   "X-Url-Scheme",
		full:   "https://host.invalid/",
		header: http.Header{"X-Url-Scheme": []string{"https"}},
	}, {
		name:   "Front-End-Https",
		full:   "https://host.invalid/",
		header: http.Header{"Front-End-Https": []string{"on"}},
	}, {
		name:   "X-Forwarded-Ssl",
		full:   "https://host.invalid/",
		header: http.Header{"X-Forwarded-Ssl": []string{"on"}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := mustNewRequest("/")
			req.Host = "host.invalid"
			req.Header = test.header

			out := apmhttputil.RequestURL(req, test.forwarded)

			// Marshal the URL to gets its "full" representation.
			var w fastjson.Writer
			out.MarshalFastJSON(&w)

			var decoded struct {
				Full string
			}
			err := json.Unmarshal(w.Bytes(), &decoded)
			assert.NoError(t, err)
			assert.Equal(t, test.full, decoded.Full)
		})
	}
}

func mustNewRequest(url string) *http.Request {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	return req
}
