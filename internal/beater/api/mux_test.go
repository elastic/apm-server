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

package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/sync/semaphore"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestBackendRequestMetadata(t *testing.T) {
	tNow := time.Now().UTC()
	c := &request.Context{Timestamp: tNow}
	cfg := &config.Config{AugmentEnabled: true}
	event := backendRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, modelpb.ToTime(event.Timestamp))
	assert.Nil(t, nil, event.Host)

	c.ClientIP = netip.MustParseAddr("127.0.0.1")
	event = backendRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, modelpb.ToTime(event.Timestamp))
	assert.Equal(t, &modelpb.Host{Ip: []*modelpb.IP{modelpb.Addr2IP(c.ClientIP)}}, event.Host)
}

func TestRUMRequestMetadata(t *testing.T) {
	tNow := time.Now().UTC()
	c := &request.Context{Timestamp: tNow}
	cfg := &config.Config{AugmentEnabled: true}
	event := rumRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, modelpb.ToTime(event.Timestamp))
	assert.Nil(t, event.Client)
	assert.Nil(t, event.Source)
	assert.Nil(t, event.UserAgent)

	ip := netip.MustParseAddr("127.0.0.1")
	c = &request.Context{Timestamp: tNow, ClientIP: ip, SourceIP: ip, UserAgent: "firefox"}
	event = rumRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, modelpb.ToTime(event.Timestamp))
	assert.Equal(t, &modelpb.Client{Ip: modelpb.Addr2IP(c.ClientIP)}, event.Client)
	assert.Equal(t, &modelpb.Source{Ip: modelpb.Addr2IP(c.SourceIP)}, event.Source)
	assert.Equal(t, &modelpb.UserAgent{Original: c.UserAgent}, event.UserAgent)
}

func requestToMuxerWithPattern(tb testing.TB, cfg *config.Config, pattern string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(http.MethodPost, pattern, nil)
	return requestToMuxer(tb, cfg, r)
}

func requestToMuxerWithHeader(tb testing.TB, cfg *config.Config, pattern string, method string, header map[string]string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(method, pattern, nil)
	return requestToMuxer(tb, cfg, requestWithHeader(r, header))
}

func requestWithHeader(r *http.Request, header map[string]string) *http.Request {
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return r
}

func requestWithQueryString(r *http.Request, queryString map[string]string) *http.Request {
	m := r.URL.Query()
	for k, v := range queryString {
		m.Set(k, v)
	}
	r.URL.RawQuery = m.Encode()
	return r
}

func requestToMuxerWithHeaderAndQueryString(
	tb testing.TB,
	cfg *config.Config,
	pattern, method string,
	header, queryString map[string]string,
) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(method, pattern, nil)
	r = requestWithQueryString(r, queryString)
	r = requestWithHeader(r, header)
	return requestToMuxer(tb, cfg, r)
}

func requestToMuxer(tb testing.TB, cfg *config.Config, r *http.Request) (*httptest.ResponseRecorder, error) {
	_, mux, err := muxBuilder{}.build(tb, cfg)
	if err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w, nil
}

func testPanicMiddleware(t *testing.T, urlPath string) {
	h, _ := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)

	var rec WriterPanicOnce
	h.ServeHTTP(&rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	assert.JSONEq(t, `{"error":"panic handling request"}`, rec.Body.String())
}

func testMonitoringMiddleware(t *testing.T, urlPath string, expectedMetrics map[string]any) {
	h, reader := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	monitoringtest.ExpectContainOtelMetrics(t, reader, expectedMetrics)
}

func newTestMux(t *testing.T, cfg *config.Config) (http.Handler, sdkmetric.Reader) {
	reader, mux, err := muxBuilder{}.build(t, cfg)
	require.NoError(t, err)
	return mux, reader
}

type muxBuilder struct {
	SourcemapFetcher sourcemap.Fetcher
	Managed          bool
}

func (m muxBuilder) build(tb testing.TB, cfg *config.Config) (sdkmetric.Reader, http.Handler, error) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	nopBatchProcessor := modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error { return nil })
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	authenticator, _ := auth.NewAuthenticator(cfg.AgentAuth, logptest.NewTestingLogger(tb, ""))
	r, err := NewMux(
		cfg,
		nopBatchProcessor,
		authenticator,
		agentcfg.NewEmptyFetcher(),
		ratelimitStore,
		m.SourcemapFetcher,
		func() bool { return true },
		semaphore.NewWeighted(1),
		mp,
	)
	return reader, r, err
}

// WriterPanicOnce implements the http.ResponseWriter interface
// It panics once when any method is called.
type WriterPanicOnce struct {
	StatusCode int
	Body       bytes.Buffer
	panicked   bool
}

// Header panics if it is the first call to the struct, otherwise returns empty Header
func (w *WriterPanicOnce) Header() http.Header {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on Header"))
	}
	return http.Header{}
}

// Write panics if it is the first call to the struct, otherwise it writes the given bytes to the body
func (w *WriterPanicOnce) Write(b []byte) (int, error) {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on Write"))
	}
	return w.Body.Write(b)
}

// WriteHeader panics if it is the first call to the struct, otherwise it writes the given status code
func (w *WriterPanicOnce) WriteHeader(statusCode int) {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on WriteHeader"))
	}
	w.StatusCode = statusCode
}
