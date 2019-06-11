// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.elastic.co/apm/internal/apmconfig"
)

const (
	intakePath = "/intake/v2/events"

	envSecretToken      = "ELASTIC_APM_SECRET_TOKEN"
	envServerURLs       = "ELASTIC_APM_SERVER_URLS"
	envServerURL        = "ELASTIC_APM_SERVER_URL"
	envServerTimeout    = "ELASTIC_APM_SERVER_TIMEOUT"
	envServerCert       = "ELASTIC_APM_SERVER_CERT"
	envVerifyServerCert = "ELASTIC_APM_VERIFY_SERVER_CERT"
)

var (
	// Take a copy of the http.DefaultTransport pointer,
	// in case another package replaces the value later.
	defaultHTTPTransport = http.DefaultTransport.(*http.Transport)

	defaultServerURL, _  = url.Parse("http://localhost:8200")
	defaultServerTimeout = 30 * time.Second
)

// HTTPTransport is an implementation of Transport, sending payloads via
// a net/http client.
type HTTPTransport struct {
	// Client exposes the http.Client used by the HTTPTransport for
	// sending requests to the APM Server.
	Client  *http.Client
	headers http.Header

	intakeURLs     []*url.URL
	intakeURLIndex int
	shuffleRand    *rand.Rand
}

// NewHTTPTransport returns a new HTTPTransport which can be used for
// streaming data to the APM Server. The returned HTTPTransport will be
// initialized using the following environment variables:
//
// - ELASTIC_APM_SERVER_URLS: a comma-separated list of APM Server URLs.
//   The transport will use this list of URLs for sending requests,
//   switching to the next URL in the list upon error. The list will be
//   shuffled first. If no URLs are specified, then the transport will
//   use the default URL "http://localhost:8200".
//
// - ELASTIC_APM_SERVER_TIMEOUT: timeout for requests to the APM Server.
//   If not specified, defaults to 30 seconds.
//
// - ELASTIC_APM_SECRET_TOKEN: used to authenticate the agent.
//
// - ELASTIC_APM_SERVER_CERT: path to a PEM-encoded certificate that
//   must match the APM Server-supplied certificate. This can be used
//   to pin a self signed certificate. If this is set, then
//   ELASTIC_APM_VERIFY_SERVER_CERT is ignored.
//
// - ELASTIC_APM_VERIFY_SERVER_CERT: if set to "false", the transport
//   will not verify the APM Server's TLS certificate. Only relevant
//   when using HTTPS. By default, the transport will verify server
//   certificates.
//
func NewHTTPTransport() (*HTTPTransport, error) {
	verifyServerCert, err := apmconfig.ParseBoolEnv(envVerifyServerCert, true)
	if err != nil {
		return nil, err
	}

	serverTimeout, err := apmconfig.ParseDurationEnv(envServerTimeout, defaultServerTimeout)
	if err != nil {
		return nil, err
	}
	if serverTimeout < 0 {
		serverTimeout = 0
	}

	serverURLs, err := initServerURLs()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: !verifyServerCert}
	serverCertPath := os.Getenv(envServerCert)
	if serverCertPath != "" {
		serverCert, err := loadCertificate(serverCertPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load certificate from %s", serverCertPath)
		}
		// Disable standard verification, we'll check that the
		// server supplies the exact certificate provided.
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return verifyPeerCertificate(rawCerts, serverCert)
		}
	}

	client := &http.Client{
		Timeout: serverTimeout,
		Transport: &http.Transport{
			Proxy:                 defaultHTTPTransport.Proxy,
			DialContext:           defaultHTTPTransport.DialContext,
			MaxIdleConns:          defaultHTTPTransport.MaxIdleConns,
			IdleConnTimeout:       defaultHTTPTransport.IdleConnTimeout,
			TLSHandshakeTimeout:   defaultHTTPTransport.TLSHandshakeTimeout,
			ExpectContinueTimeout: defaultHTTPTransport.ExpectContinueTimeout,
			TLSClientConfig:       tlsConfig,
		},
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/x-ndjson")
	headers.Set("Content-Encoding", "deflate")
	headers.Set("Transfer-Encoding", "chunked")
	t := &HTTPTransport{
		Client:  client,
		headers: headers,
	}
	t.SetSecretToken(os.Getenv(envSecretToken))
	t.SetServerURL(serverURLs...)
	return t, nil
}

// SetServerURL sets the APM Server URL (or URLs) for sending requests.
// At least one URL must be specified, or the method will panic. The
// list will be randomly shuffled.
func (t *HTTPTransport) SetServerURL(u ...*url.URL) {
	if len(u) == 0 {
		panic("SetServerURL expects at least one URL")
	}
	intakeURLs := make([]*url.URL, len(u))
	for i, u := range u {
		intakeURLs[i] = urlWithPath(u, intakePath)
	}
	if n := len(intakeURLs); n > 0 {
		if t.shuffleRand == nil {
			t.shuffleRand = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		for i := n - 1; i > 0; i-- {
			j := t.shuffleRand.Intn(i + 1)
			intakeURLs[i], intakeURLs[j] = intakeURLs[j], intakeURLs[i]
		}
	}
	t.intakeURLs = intakeURLs
	t.intakeURLIndex = 0
}

// SetUserAgent sets the User-Agent header that will be sent with each request.
func (t *HTTPTransport) SetUserAgent(ua string) {
	t.headers.Set("User-Agent", ua)
}

// SetSecretToken sets the Authorization header with the given secret token.
// This overrides the value specified via the ELASTIC_APM_SECRET_TOKEN
// environment variable, if any.
func (t *HTTPTransport) SetSecretToken(secretToken string) {
	if secretToken != "" {
		t.headers.Set("Authorization", "Bearer "+secretToken)
	} else {
		t.headers.Del("Authorization")
	}
}

// SendStream sends the stream over HTTP. If SendStream returns an error and
// the transport is configured with more than one APM Server URL, then the
// following request will be sent to the next URL in the list.
func (t *HTTPTransport) SendStream(ctx context.Context, r io.Reader) error {
	intakeURL := t.intakeURLs[t.intakeURLIndex]
	req := t.newRequest(intakeURL)
	req = requestWithContext(ctx, req)
	req.Body = ioutil.NopCloser(r)
	if err := t.sendRequest(req); err != nil {
		t.intakeURLIndex = (t.intakeURLIndex + 1) % len(t.intakeURLs)
		return err
	}
	return nil
}

func (t *HTTPTransport) sendRequest(req *http.Request) error {
	resp, err := t.Client.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending request failed")
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
	result := &HTTPError{
		Response: resp,
		Message:  strings.TrimSpace(string(bodyContents)),
	}
	if resp.StatusCode == http.StatusNotFound && result.Message == "404 page not found" {
		// This may be an old (pre-6.5) APM server
		// that does not support the v2 intake API.
		result.Message = fmt.Sprintf("%s not found (requires APM Server 6.5.0 or newer)", req.URL)
	}
	return result
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
	Response *http.Response
	Message  string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("request failed with %s", e.Response.Status)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// initServerURLs parses ELASTIC_APM_SERVER_URLS if specified,
// otherwise parses ELASTIC_APM_SERVER_URL if specified. If
// neither are specified, then the default localhost URL is
// returned.
func initServerURLs() ([]*url.URL, error) {
	key := envServerURLs
	value := os.Getenv(key)
	if value == "" {
		key = envServerURL
		value = os.Getenv(key)
	}
	var urls []*url.URL
	for _, field := range strings.Split(value, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		u, err := url.Parse(field)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", key)
		}
		urls = append(urls, u)
	}
	if len(urls) == 0 {
		urls = []*url.URL{defaultServerURL}
	}
	return urls, nil
}

func requestWithContext(ctx context.Context, req *http.Request) *http.Request {
	url := req.URL
	req.URL = nil
	reqCopy := req.WithContext(ctx)
	reqCopy.URL = url
	req.URL = url
	return reqCopy
}

func loadCertificate(path string) (*x509.Certificate, error) {
	pemBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for {
		var certBlock *pem.Block
		certBlock, pemBytes = pem.Decode(pemBytes)
		if certBlock == nil {
			return nil, errors.New("missing or invalid certificate")
		}
		if certBlock.Type == "CERTIFICATE" {
			return x509.ParseCertificate(certBlock.Bytes)
		}
	}
}

func verifyPeerCertificate(rawCerts [][]byte, trusted *x509.Certificate) error {
	if len(rawCerts) == 0 {
		return errors.New("missing leaf certificate")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return errors.Wrap(err, "failed to parse certificate from server")
	}
	if !cert.Equal(trusted) {
		return errors.New("failed to verify server certificate")
	}
	return nil
}
