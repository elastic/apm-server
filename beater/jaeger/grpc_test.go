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
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
)

func TestGRPCCollector_PostSpans(t *testing.T) {
	for name, tc := range map[string]testGRPCCollector{
		"empty request": {
			request: &api_v2.PostSpansRequest{},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:       1,
				request.IDResponseCount:      1,
				request.IDResponseValidCount: 1,
			},
		},
		"successful request": {
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:       1,
				request.IDResponseCount:      1,
				request.IDResponseValidCount: 1,
				request.IDEventReceivedCount: 2,
			},
		},
		"failing request": {
			consumerErr: errors.New("consumer failed"),
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:        1,
				request.IDResponseCount:       1,
				request.IDResponseErrorsCount: 1,
				request.IDEventReceivedCount:  2,
			},
		},
		"auth fails": {
			authError: errors.New("oh noes"),
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsUnauthorized: 1,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)

			expectedErr := tc.authError
			if expectedErr == nil {
				expectedErr = tc.consumerErr
			}
			resp, err := tc.collector.PostSpans(context.Background(), tc.request)
			if expectedErr != nil {
				require.Nil(t, resp)
				require.Error(t, err)
				assert.Equal(t, expectedErr, err)
			} else {
				require.NotNil(t, resp)
				require.NoError(t, err)
			}
			assertMonitoring(t, tc.monitoringInt, gRPCMonitoringMap)
		})
	}
}

type testGRPCCollector struct {
	request     *api_v2.PostSpansRequest
	authError   error
	consumerErr error
	collector   grpcCollector

	monitoringInt map[request.ResultID]int64
}

func (tc *testGRPCCollector) setup(t *testing.T) {
	beatertest.ClearRegistry(gRPCMonitoringMap)
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

	tc.collector = grpcCollector{authFunc(func(context.Context, model.Batch) error {
		return tc.authError
	}), traceConsumerFunc(func(ctx context.Context, td consumerdata.TraceData) error {
		return tc.consumerErr
	})}
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

type traceConsumerFunc func(ctx context.Context, td consumerdata.TraceData) error

func (f traceConsumerFunc) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	return f(ctx, td)
}

func nopConsumer() traceConsumerFunc {
	return func(context.Context, consumerdata.TraceData) error { return nil }
}
