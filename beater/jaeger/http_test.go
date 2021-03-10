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

package jaeger

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	jaegerthrift "github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
)

type httpMuxTest struct {
	spans         []*jaegerthrift.Span
	consumerError error

	expectedStatusCode    int
	expectedMonitoringMap map[request.ResultID]int64
}

func TestHTTPMux(t *testing.T) {
	for name, test := range map[string]httpMuxTest{
		"empty batch": {
			expectedStatusCode: http.StatusAccepted,
			expectedMonitoringMap: map[request.ResultID]int64{
				request.IDRequestCount:       1,
				request.IDResponseCount:      1,
				request.IDResponseValidCount: 1,
				request.IDEventReceivedCount: 0,
			},
		},
		"non-empty batch": {
			spans:              []*jaegerthrift.Span{{}, {}},
			expectedStatusCode: http.StatusAccepted,
			expectedMonitoringMap: map[request.ResultID]int64{
				request.IDRequestCount:       1,
				request.IDResponseCount:      1,
				request.IDResponseValidCount: 1,
				request.IDEventReceivedCount: 2,
			},
		},
		"consumer fails": {
			spans:              []*jaegerthrift.Span{{}, {}},
			consumerError:      errors.New("oh noes"),
			expectedStatusCode: http.StatusInternalServerError,
			expectedMonitoringMap: map[request.ResultID]int64{
				request.IDRequestCount:        1,
				request.IDResponseCount:       1,
				request.IDResponseErrorsCount: 1,
				request.IDEventReceivedCount:  2,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			testHTTPMux(t, test)
		})
	}
}

func testHTTPMux(t *testing.T, test httpMuxTest) {
	t.Helper()
	beatertest.ClearRegistry(httpMonitoringMap)

	var consumed bool
	mux, err := newHTTPMux(tracesConsumerFunc(func(ctx context.Context, _ pdata.Traces) error {
		consumed = true
		return test.consumerError
	}))
	require.NoError(t, err)

	body := encodeThriftSpans(test.spans...)
	req := httptest.NewRequest("POST", "/api/traces", body)
	req.Header.Set("Content-Type", "application/x-thrift")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	assert.Equal(t, test.expectedStatusCode, recorder.Code)
	assert.True(t, consumed)
	assertMonitoring(t, test.expectedMonitoringMap, httpMonitoringMap)
}

func assertMonitoring(t *testing.T, expected map[request.ResultID]int64, actual monitoringMap) {
	for _, k := range monitoringKeys {
		if val, ok := expected[k]; ok {
			assert.Equalf(t, val, actual[k].Get(), "%s mismatch", k)
		} else {
			assert.Zerof(t, actual[k].Get(), "%s mismatch", k)
		}
	}
}

func TestHTTPHandler_UnknownRoute(t *testing.T) {
	c, recorder := newRequestContext("POST", "/foo", nil)
	newHTTPHandler(nopConsumer())(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, `{"error":"404 page not found: unknown route"}`+"\n", recorder.Body.String())
}

func TestHTTPMux_MethodNotAllowed(t *testing.T) {
	c, recorder := newRequestContext("GET", "/api/traces", nil)
	newHTTPHandler(nopConsumer())(c)
	assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
	assert.Equal(t, `{"error":"method not supported: only POST requests are allowed"}`+"\n", recorder.Body.String())
}

func TestHTTPMux_InvalidContentType(t *testing.T) {
	c, recorder := newRequestContext("POST", "/api/traces", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	newHTTPHandler(nopConsumer())(c)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, `{"error":"data validation error: unsupported content-type \"application/json\""}`+"\n", recorder.Body.String())
}

func TestHTTPMux_ValidContentTypes(t *testing.T) {
	for _, contentType := range []string{"application/x-thrift", "application/vnd.apache.thrift.binary"} {
		body := encodeThriftSpans(&jaegerthrift.Span{})
		c, recorder := newRequestContext("POST", "/api/traces", body)
		c.Request.Header.Set("Content-Type", contentType)
		newHTTPHandler(nopConsumer())(c)
		assert.Equal(t, http.StatusAccepted, recorder.Code)
		assert.Equal(t, ``, recorder.Body.String())
	}
}

func TestHTTPMux_InvalidBody(t *testing.T) {
	c, recorder := newRequestContext("POST", "/api/traces", strings.NewReader(`¯\_(ツ)_/¯`))
	newHTTPHandler(nopConsumer())(c)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Regexp(t, `{"error":"data decoding error: .*"}`+"\n", recorder.Body.String())
}

func TestHTTPMux_ConsumerError(t *testing.T) {
	var consumer tracesConsumerFunc = func(ctx context.Context, _ pdata.Traces) error {
		return errors.New("bauch tut weh")
	}
	c, recorder := newRequestContext("POST", "/api/traces", encodeThriftSpans(&jaegerthrift.Span{}))
	newHTTPHandler(consumer)(c)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Regexp(t, `{"error":"internal error: bauch tut weh"}`+"\n", recorder.Body.String())
}

func newRequestContext(method, path string, body io.Reader) (*request.Context, *httptest.ResponseRecorder) {
	rr := httptest.NewRecorder()
	c := request.NewContext()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/x-thrift")
	c.Reset(rr, req)
	return c, rr
}

func encodeThriftSpans(spans ...*jaegerthrift.Span) io.Reader {
	batch := &jaegerthrift.Batch{Process: &jaegerthrift.Process{ServiceName: "whatever"}, Spans: spans}
	return encodeThriftBatch(batch)
}

func encodeThriftBatch(batch *jaegerthrift.Batch) io.Reader {
	transport := thrift.NewTMemoryBuffer()
	if err := batch.Write(thrift.NewTBinaryProtocolTransport(transport)); err != nil {
		panic(err)
	}
	return bytes.NewReader(transport.Buffer.Bytes())
}
