package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	"github.com/elastic/apm-agent-go/model"
)

const (
	transactionsPath = "/v1/transactions"
	errorsPath       = "/v1/errors"
	metricsPath      = "/v1/metrics"

	envSecretToken      = "ELASTIC_APM_SECRET_TOKEN"
	envServerURL        = "ELASTIC_APM_SERVER_URL"
	envVerifyServerCert = "ELASTIC_APM_VERIFY_SERVER_CERT"
)

var (
	// Take a copy of the http.DefaultTransport pointer,
	// in case another package replaces the value later.
	defaultHTTPTransport = http.DefaultTransport.(*http.Transport)
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
	jsonwriter      fastjson.Writer
}

// NewHTTPTransport returns a new HTTPTransport, which can be used for sending
// transactions and errors to the APM server at the specified URL, with the
// given secret token.
//
// If the URL specified is the empty string, then NewHTTPTransport will use the
// value of the ELASTIC_APM_SERVER_URL environment variable, if defined; if
// the environment variable is also undefined, then an error will be returned.
// The URL must be the base server URL, excluding any transactions or errors
// path. e.g. "http://elastic-apm.example:8200".
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
			return nil, errors.Errorf(
				"URL not specified, and $%s not set", envServerURL,
			)
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

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	if secretToken == "" {
		secretToken = os.Getenv(envSecretToken)
	}
	if secretToken != "" {
		headers.Set("Authorization", "Bearer "+secretToken)
	}

	return &HTTPTransport{
		Client:          client,
		baseURL:         req.URL,
		transactionsURL: urlWithPath(req.URL, transactionsPath),
		errorsURL:       urlWithPath(req.URL, errorsPath),
		metricsURL:      urlWithPath(req.URL, metricsPath),
		headers:         headers,
	}, nil
}

// SendTransactions sends the transactions payload over HTTP.
func (t *HTTPTransport) SendTransactions(ctx context.Context, p *model.TransactionsPayload) error {
	t.jsonwriter.Reset()
	p.MarshalFastJSON(&t.jsonwriter)
	buf := t.jsonwriter.Bytes()

	req := t.newTransactionsRequest().WithContext(ctx)
	req.ContentLength = int64(len(buf))
	req.Body = ioutil.NopCloser(bytes.NewReader(buf))
	return t.send(req, "SendTransactions")
}

// SendErrors sends the errors payload over HTTP.
func (t *HTTPTransport) SendErrors(ctx context.Context, p *model.ErrorsPayload) error {
	t.jsonwriter.Reset()
	p.MarshalFastJSON(&t.jsonwriter)
	buf := t.jsonwriter.Bytes()

	req := t.newErrorsRequest().WithContext(ctx)
	req.ContentLength = int64(len(buf))
	req.Body = ioutil.NopCloser(bytes.NewReader(buf))
	return t.send(req, "SendErrors")
}

// SendMetrics sends the metrics payload over HTTP.
func (t *HTTPTransport) SendMetrics(ctx context.Context, p *model.MetricsPayload) error {
	t.jsonwriter.Reset()
	p.MarshalFastJSON(&t.jsonwriter)
	buf := t.jsonwriter.Bytes()

	req := t.newMetricsRequest().WithContext(ctx)
	req.ContentLength = int64(len(buf))
	req.Body = ioutil.NopCloser(bytes.NewReader(buf))
	return t.send(req, "SendMetrics")
}

func (t *HTTPTransport) send(req *http.Request, op string) error {
	resp, err := t.Client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "sending request for %s failed", op)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		return nil
	}

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
