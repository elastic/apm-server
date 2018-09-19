package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/apmconfig"
	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

const (
	transactionsPath = "/v1/transactions"
	errorsPath       = "/v1/errors"
	metricsPath      = "/v1/metrics"

	envSecretToken      = "ELASTIC_APM_SECRET_TOKEN"
	envServerURL        = "ELASTIC_APM_SERVER_URL"
	envServerTimeout    = "ELASTIC_APM_SERVER_TIMEOUT"
	envVerifyServerCert = "ELASTIC_APM_VERIFY_SERVER_CERT"

	// gzipThresholdBytes is the minimum size of the uncompressed
	// payload before we'll consider gzip-compressing it.
	gzipThresholdBytes = 1024
)

var (
	// Take a copy of the http.DefaultTransport pointer,
	// in case another package replaces the value later.
	defaultHTTPTransport = http.DefaultTransport.(*http.Transport)

	defaultServerURL     = "http://localhost:8200"
	defaultServerTimeout = 30 * time.Second
)

// HTTPTransport is an implementation of Transport, sending payloads via
// a net/http client.
type HTTPTransport struct {
	Client          *http.Client
	baseURL         *url.URL
	transactionsURL *url.URL
	errorsURL       *url.URL
	metricsURL      *url.URL
	headers         http.Header
	gzipHeaders     http.Header
	jsonWriter      fastjson.Writer
	gzipWriter      *gzip.Writer
	gzipBuffer      bytes.Buffer
}

// NewHTTPTransport returns a new HTTPTransport, which can be used for sending
// transactions and errors to the APM server at the specified URL, with the
// given secret token.
//
// If the URL specified is the empty string, then NewHTTPTransport will use the
// value of the ELASTIC_APM_SERVER_URL environment variable, if defined; if
// the environment variable is also undefined, then the transport will use the
// default URL "http://localhost:8200". The URL must be the base server URL,
// excluding any transactions or errors path. e.g. "http://server.example:8200".
//
// If the secret token specified is the empty string, then NewHTTPTransport
// will use the value of the ELASTIC_APM_SECRET_TOKEN environment variable, if
// defined; if the environment variable is also undefined, then requests will
// not be authenticated.
//
// If ELASTIC_APM_VERIFY_SERVER_CERT is set to "false", then the transport
// will not verify the APM server's TLS certificate.
//
// The Client field will be initialized with a new http.Client configured from
// ELASTIC_APM_* environment variables. The Client field may be modified or
// replaced, e.g. in order to specify TLS root CAs.
func NewHTTPTransport(serverURL, secretToken string) (*HTTPTransport, error) {
	if serverURL == "" {
		serverURL = os.Getenv(envServerURL)
		if serverURL == "" {
			serverURL = defaultServerURL
		}
	}
	req, err := http.NewRequest("POST", serverURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if req.URL.Scheme == "https" && os.Getenv(envVerifyServerCert) == "false" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		client.Transport = &http.Transport{
			Proxy:                 defaultHTTPTransport.Proxy,
			DialContext:           defaultHTTPTransport.DialContext,
			MaxIdleConns:          defaultHTTPTransport.MaxIdleConns,
			IdleConnTimeout:       defaultHTTPTransport.IdleConnTimeout,
			TLSHandshakeTimeout:   defaultHTTPTransport.TLSHandshakeTimeout,
			ExpectContinueTimeout: defaultHTTPTransport.ExpectContinueTimeout,
			TLSClientConfig:       tlsConfig,
		}
	}

	timeout, err := apmconfig.ParseDurationEnv(envServerTimeout, "s", defaultServerTimeout)
	if err != nil {
		return nil, err
	}
	if timeout > 0 {
		client.Timeout = timeout
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if secretToken == "" {
		secretToken = os.Getenv(envSecretToken)
	}
	if secretToken != "" {
		headers.Set("Authorization", "Bearer "+secretToken)
	}

	gzipHeaders := make(http.Header)
	for k, v := range headers {
		gzipHeaders[k] = v
	}
	gzipHeaders.Set("Content-Encoding", "gzip")

	t := &HTTPTransport{
		Client:          client,
		baseURL:         req.URL,
		transactionsURL: urlWithPath(req.URL, transactionsPath),
		errorsURL:       urlWithPath(req.URL, errorsPath),
		metricsURL:      urlWithPath(req.URL, metricsPath),
		headers:         headers,
		gzipHeaders:     gzipHeaders,
	}
	t.gzipWriter = gzip.NewWriter(&t.gzipBuffer)
	return t, nil
}

// SetUserAgent sets the User-Agent header that will be
// sent with each request.
func (t *HTTPTransport) SetUserAgent(ua string) {
	t.headers.Set("User-Agent", ua)
	t.gzipHeaders.Set("User-Agent", ua)
}

// SendTransactions sends the transactions payload over HTTP.
func (t *HTTPTransport) SendTransactions(ctx context.Context, p *model.TransactionsPayload) error {
	t.jsonWriter.Reset()
	p.MarshalFastJSON(&t.jsonWriter)
	req := requestWithContext(ctx, t.newTransactionsRequest())
	return t.sendPayload(req, "SendTransactions")
}

// SendErrors sends the errors payload over HTTP.
func (t *HTTPTransport) SendErrors(ctx context.Context, p *model.ErrorsPayload) error {
	t.jsonWriter.Reset()
	p.MarshalFastJSON(&t.jsonWriter)
	req := requestWithContext(ctx, t.newErrorsRequest())
	return t.sendPayload(req, "SendErrors")
}

// SendMetrics sends the metrics payload over HTTP.
func (t *HTTPTransport) SendMetrics(ctx context.Context, p *model.MetricsPayload) error {
	t.jsonWriter.Reset()
	p.MarshalFastJSON(&t.jsonWriter)
	req := requestWithContext(ctx, t.newMetricsRequest())
	return t.sendPayload(req, "SendMetrics")
}

func (t *HTTPTransport) sendPayload(req *http.Request, op string) error {
	buf := t.jsonWriter.Bytes()
	var body io.Reader = bytes.NewReader(buf)
	req.ContentLength = int64(len(buf))
	if req.ContentLength >= gzipThresholdBytes {
		t.gzipBuffer.Reset()
		t.gzipWriter.Reset(&t.gzipBuffer)
		if _, err := io.Copy(t.gzipWriter, body); err != nil {
			return err
		}
		if err := t.gzipWriter.Flush(); err != nil {
			return err
		}
		req.ContentLength = int64(t.gzipBuffer.Len())
		body = &t.gzipBuffer
		req.Header = t.gzipHeaders
	}
	req.Body = ioutil.NopCloser(body)

	resp, err := t.Client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "sending request for %s failed", op)
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		resp.Body.Close()
		return nil
	}
	defer resp.Body.Close()

	// apm-server will return 503 Service Unavailable
	// if the data cannot be published to Elasticsearch,
	// but there is no Retry-After header included, so
	// we treat it as any other internal server error.
	bodyContents, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		resp.Body = ioutil.NopCloser(bytes.NewReader(bodyContents))
	}
	return &HTTPError{
		Op:       op,
		Response: resp,
		Message:  strings.TrimSpace(string(bodyContents)),
	}
}

func (t *HTTPTransport) newTransactionsRequest() *http.Request {
	return t.newRequest(t.transactionsURL)
}

func (t *HTTPTransport) newErrorsRequest() *http.Request {
	return t.newRequest(t.errorsURL)
}

func (t *HTTPTransport) newMetricsRequest() *http.Request {
	return t.newRequest(t.metricsURL)
}

func (t *HTTPTransport) newRequest(url *url.URL) *http.Request {
	req := &http.Request{
		Method:     "POST",
		URL:        url,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     t.headers,
		Host:       url.Host,
	}
	return req
}

func urlWithPath(url *url.URL, p string) *url.URL {
	urlCopy := *url
	urlCopy.Path += p
	if urlCopy.RawPath != "" {
		urlCopy.RawPath += p
	}
	return &urlCopy
}

// HTTPError is an error returned by HTTPTransport methods when requests fail.
type HTTPError struct {
	Op       string
	Response *http.Response
	Message  string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("%s failed with %s", e.Op, e.Response.Status)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func requestWithContext(ctx context.Context, req *http.Request) *http.Request {
	url := req.URL
	req.URL = nil
	reqCopy := req.WithContext(ctx)
	reqCopy.URL = url
	req.URL = url
	return reqCopy
}
