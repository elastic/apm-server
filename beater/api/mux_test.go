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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/monitoring"
)

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
	nopReporter := func(context.Context, publish.PendingReq) error { return nil }
	nopBatchProcessor := model.ProcessBatchFunc(func(context.Context, *model.Batch) error { return nil })
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	mux, err := NewMux(beat.Info{Version: "1.2.3"}, cfg, nopReporter, nopBatchProcessor, agentcfg.NewFetcher(cfg), ratelimitStore)
	if err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	h, _ := mux.Handler(r)
	h.ServeHTTP(w, r)
	return w, nil
}

func testPanicMiddleware(t *testing.T, urlPath string, approvalPath string) {
	h := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)

	var rec beatertest.WriterPanicOnce
	h.ServeHTTP(&rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.StatusCode)
	approvaltest.ApproveJSON(t, approvalPath, rec.Body.Bytes())
}

func testMonitoringMiddleware(t *testing.T, urlPath string, monitoringMap map[request.ResultID]*monitoring.Int, expected map[request.ResultID]int) {
	beatertest.ClearRegistry(monitoringMap)

	h := newTestMux(t, config.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, urlPath, nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	equal, result := beatertest.CompareMonitoringInt(expected, monitoringMap)
	assert.True(t, equal, result)
}

func newTestMux(t *testing.T, cfg *config.Config) http.Handler {
	nopReporter := func(context.Context, publish.PendingReq) error { return nil }
	nopBatchProcessor := model.ProcessBatchFunc(func(context.Context, *model.Batch) error { return nil })
	ratelimitStore, _ := ratelimit.NewStore(1000, 1000, 1000)
	mux, err := NewMux(beat.Info{Version: "1.2.3"}, cfg, nopReporter, nopBatchProcessor, agentcfg.NewFetcher(cfg), ratelimitStore)
	require.NoError(t, err)
	return mux
}
