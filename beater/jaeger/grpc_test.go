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
	"strings"
	"testing"
	"time"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/tests"
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

			var expectedErr error
			if tc.authError != nil {
				expectedErr = status.Error(codes.Unauthenticated, tc.authError.Error())
			} else {
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
			assertMonitoring(t, tc.monitoringInt, gRPCCollectorMonitoringMap)
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
	beatertest.ClearRegistry(gRPCCollectorMonitoringMap)
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

	tc.collector = grpcCollector{logp.NewLogger("gRPC"), authFunc(func(context.Context, model.Batch) error {
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

func TestGRPCSampler_GetSamplingStrategy(t *testing.T) {
	for name, tc := range map[string]testGRPCSampler{
		"withSamplingRate": {
			expectedSamplingRate: 0.75},
		"noSamplingRate": {
			kibanaBody: map[string]interface{}{
				"_id": "1",
				"_source": map[string]interface{}{
					"settings": map[string]interface{}{}}},
			expectedErrMsg: "no sampling rate found",
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:           1,
				request.IDResponseCount:          1,
				request.IDResponseErrorsCount:    1,
				request.IDResponseErrorsNotFound: 1}},
		"invalidSamplingRate": {
			kibanaBody: map[string]interface{}{
				"_id": "1",
				"_source": map[string]interface{}{
					"settings": map[string]interface{}{
						agentcfg.TransactionSamplingRateKey: "foo"}}},
			expectedErrMsg: "parsing error for sampling rate",
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:           1,
				request.IDResponseCount:          1,
				request.IDResponseErrorsCount:    1,
				request.IDResponseErrorsInternal: 1}},
		"unsupportedVersion": {
			kibanaVersion:  common.MustNewVersion("7.4.0"),
			expectedErrMsg: "not supported by Kibana version",
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:                     1,
				request.IDResponseCount:                    1,
				request.IDResponseErrorsCount:              1,
				request.IDResponseErrorsServiceUnavailable: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
			tc.setup()
			params := &api_v2.SamplingStrategyParameters{ServiceName: "serviceA"}
			resp, err := tc.sampler.GetSamplingStrategy(context.Background(), params)

			// assert sampling response
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				assert.Nil(t, resp)
				logs := func() string {
					var sb strings.Builder
					for _, entry := range logp.ObserverLogs().All() {
						sb.WriteString(entry.Message)
					}
					return sb.String()
				}()
				assert.Contains(t, logs, "Fetching sampling rate failed")
			} else {
				require.NoError(t, err)
				require.Equal(t, api_v2.SamplingStrategyType_PROBABILISTIC, resp.StrategyType)
				assert.Equal(t, tc.expectedSamplingRate, resp.ProbabilisticSampling.SamplingRate)
				assert.Nil(t, resp.OperationSampling)
				assert.Nil(t, resp.RateLimitingSampling)
			}

			// assert monitoring counters
			assertMonitoring(t, tc.monitoringInt, gRPCSamplingMonitoringMap)
		})
	}
}

type testGRPCSampler struct {
	kibanaBody          map[string]interface{}
	kibanaCode          int
	kibanaVersion       *common.Version
	sampler             grpcSampler
	defaultSamplingRate float64

	expectedErrMsg       string
	expectedSamplingRate float64
	monitoringInt        map[request.ResultID]int64
}

func (tc *testGRPCSampler) setup() {
	tc.defaultSamplingRate = 1.0
	if tc.expectedSamplingRate == 0.0 {
		tc.expectedSamplingRate = tc.defaultSamplingRate
	}
	if tc.kibanaCode == 0 {
		tc.kibanaCode = 200
	}
	if tc.kibanaBody == nil {
		tc.kibanaBody = map[string]interface{}{
			"_id": "1",
			"_source": map[string]interface{}{
				"settings": map[string]interface{}{
					agentcfg.TransactionSamplingRateKey: 0.75,
				},
			},
		}
	}
	if tc.kibanaVersion == nil {
		tc.kibanaVersion = common.MustNewVersion("7.7.0")
	}
	client := tests.MockKibana(tc.kibanaCode, tc.kibanaBody, *tc.kibanaVersion, true)
	fetcher := agentcfg.NewFetcher(client, time.Second)
	tc.sampler = grpcSampler{logp.L(), tc.defaultSamplingRate, client, fetcher}
	beatertest.ClearRegistry(gRPCSamplingMonitoringMap)
	if tc.monitoringInt == nil {
		tc.monitoringInt = map[request.ResultID]int64{
			request.IDRequestCount:       1,
			request.IDResponseCount:      1,
			request.IDResponseValidCount: 1,
		}
	}
}
