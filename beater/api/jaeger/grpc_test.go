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
	"context"
	"errors"
	"testing"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/apm-server/publish"
)

func TestGRPCCollector_PostSpans(t *testing.T) {
	for name, tc := range map[string]testCollector{
		"empty request": {
			request: &api_v2.PostSpansRequest{},
			monitoringInt: map[request.ResultID]int64{request.IDRequestCount: 1, request.IDResponseCount: 1,
				request.IDResponseValidCount: 1}},
		"successful request": {
			monitoringInt: map[request.ResultID]int64{request.IDRequestCount: 1, request.IDResponseCount: 1,
				request.IDResponseValidCount: 1, request.IDEventReceivedCount: 2}},
		"failing request": {
			reporter: func(ctx context.Context, pub publish.PendingReq) error {
				return errors.New("consumer failed")
			},
			expectedErr: errors.New("consumer failed"),
			monitoringInt: map[request.ResultID]int64{request.IDRequestCount: 1, request.IDResponseCount: 1,
				request.IDResponseErrorsCount: 1, request.IDEventReceivedCount: 2}},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)

			resp, err := tc.collector.PostSpans(context.Background(), tc.request)
			if tc.expectedErr != nil {
				require.Nil(t, resp)
				require.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				require.NotNil(t, resp)
				require.NoError(t, err)
			}
			assertMonitoring(t, tc.monitoringInt)
		})
	}
}

type testCollector struct {
	request   *api_v2.PostSpansRequest
	reporter  func(ctx context.Context, pub publish.PendingReq) error
	collector GRPCCollector

	monitoringInt map[request.ResultID]int64
	expectedErr   error
}

func (tc *testCollector) setup(t *testing.T) {
	beatertest.ClearRegistry(monitoringMap)
	if tc.request == nil {
		td := consumerdata.TraceData{Spans: []*v1.Span{
			{TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				SpanId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
			{TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				SpanId: []byte{0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}}}
		batch, err := jaeger.OCProtoToJaegerProto(td)
		require.NoError(t, err)
		require.NotNil(t, batch)
		tc.request = &api_v2.PostSpansRequest{Batch: *batch}
	}

	if tc.reporter == nil {
		tc.reporter = func(ctx context.Context, pub publish.PendingReq) error { return nil }
	}
	tc.collector = GRPCCollector{&otel.Consumer{Reporter: tc.reporter}}
}

func assertMonitoring(t *testing.T, m map[request.ResultID]int64) {
	for _, k := range monitoringKeys {
		if val, ok := m[k]; ok {
			assert.Equal(t, val, monitoringMap[k].Get())
		} else {
			assert.Zero(t, monitoringMap[k].Get())
		}
	}
}
