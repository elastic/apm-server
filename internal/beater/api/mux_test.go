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

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

func TestBackendRequestMetadata(t *testing.T) {
	tNow := time.Now()
	c := &request.Context{Timestamp: tNow}
	cfg := &config.Config{AugmentEnabled: true}
	event := backendRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, event.Timestamp)
	assert.Equal(t, model.Host{}, event.Host)

	c.ClientIP = netip.MustParseAddr("127.0.0.1")
	event = backendRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, event.Timestamp)
	assert.NotEqual(t, model.Host{}, event.Host)
}

func TestRUMRequestMetadata(t *testing.T) {
	tNow := time.Now()
	c := &request.Context{Timestamp: tNow}
	cfg := &config.Config{AugmentEnabled: true}
	event := rumRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, event.Timestamp)
	assert.Equal(t, model.Client{}, event.Client)
	assert.Equal(t, model.Source{}, event.Source)
	assert.Equal(t, model.UserAgent{}, event.UserAgent)

	ip := netip.MustParseAddr("127.0.0.1")
	c = &request.Context{Timestamp: tNow, ClientIP: ip, SourceIP: ip, UserAgent: "firefox"}
	event = rumRequestMetadataFunc(cfg)(c)
	assert.Equal(t, tNow, event.Timestamp)
	assert.NotEqual(t, model.Client{}, event.Client)
	assert.NotEqual(t, model.Source{}, event.Source)
	assert.NotEqual(t, model.UserAgent{}, event.UserAgent)
}

func requestToMuxerWithPattern(cfg *config.Config, pattern string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(http.MethodPost, pattern, nil)
	return requestToMuxer(cfg, r)
}

func requestToMuxerWithHeader(cfg *config.Config, pattern string, method string, header map[string]string) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(method, pattern, nil)
	return requestToMuxer(cfg, requestWithHeader(r, header))
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
	cfg *config.Config,
	pattern, method string,
	header, queryString map[string]string,
) (*httptest.ResponseRecorder, error) {
	r := httptest.NewRequest(method, pattern, nil)
	r = requestWithQueryString(r, queryString)
	r = requestWithHeader(r, header)
	return requestToMuxer(cfg, r)
}

func requestToMuxer(cfg *config.Config, r *http.Request) (*httptest.ResponseRecorder, error) {
	mux, err := muxBuilder{}.build(cfg)
	if err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w, nil
}

func testPanicMiddleware(t *testing.T, urlPath string) {
	h := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)

	var rec WriterPanicOnce
	h.ServeHTTP(&rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	assert.JSONEq(t, `{"error":"panic handling request"}`, rec.Body.String())
}

func testMonitoringMiddleware(t *testing.T, urlPath string, monitoringMap map[request.ResultID]*monitoring.Int, expected map[request.ResultID]int) {
	monitoringtest.ClearRegistry(monitoringMap)

	h := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	equal, result := monitoringtest.CompareMonitoringInt(expected, monitoringMap)
	assert.True(t, equal, result)
}

func newTestMux(t *testing.T, cfg *config.Config) http.Handler {
	mux, err := muxBuilder{}.build(cfg)
	require.NoError(t, err)
	return mux
}

type muxBuilder struct {
	SourcemapFetcher sourcemap.Fetcher
	Managed          bool
}

func (m muxBuilder) build(cfg *config.Config) (http.Handler, error) {
	nopBatchProcessor := model.ProcessBatchFunc(func(context.Context, *model.Batch) error { return nil })
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	authenticator, _ := auth.NewAuthenticator(cfg.AgentAuth)
	return NewMux(
		cfg,
		nopBatchProcessor,
		authenticator,
		agentcfg.NewDirectFetcher(nil),
		ratelimitStore,
		m.SourcemapFetcher,
		m.Managed,
		func() bool { return true },
	)
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
