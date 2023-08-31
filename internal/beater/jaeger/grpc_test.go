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
	"net"
	"testing"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
)

func TestPostSpans(t *testing.T) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	var processorErr error
	var processor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		return processorErr
	}
	conn, _ := newServer(t, processor, nil)

	client := api_v2.NewCollectorServiceClient(conn)
	result, err := client.PostSpans(context.Background(), &api_v2.PostSpansRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	expectMetrics(t, reader, map[string]int64{
		"beats_stats.metrics.apm-server.jaeger.grpc.collect.event.received.count": 0,
		"beats_stats.metrics.apm-server.jaeger.grpc.collect.request.count":        1,
		"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.count":       1,
		"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.valid.count": 1,
	})

	type testcase struct {
		request      *api_v2.PostSpansRequest
		processorErr error
		expectedErr  error

		expectedMetrics map[string]int64
	}

	for name, tc := range map[string]testcase{
		"empty request": {
			request: &api_v2.PostSpansRequest{},

			expectedMetrics: map[string]int64{
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.event.received.count": 0,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.request.count":        1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.count":       1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.valid.count": 1,
			},
		},
		"successful request": {
			request: newPostSpansRequest(t),

			expectedMetrics: map[string]int64{
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.event.received.count": 2,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.request.count":        1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.count":       1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.valid.count": 1,
			},
		},
		"failing request": {
			request:      newPostSpansRequest(t),
			processorErr: errors.New("processor failed"),
			expectedErr:  status.Error(codes.Unknown, "processor failed"),

			expectedMetrics: map[string]int64{
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.event.received.count":  2,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.request.count":         1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.count":        1,
				"beats_stats.metrics.apm-server.jaeger.grpc.collect.response.errors.count": 1,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			processorErr = tc.processorErr
			resp, err := client.PostSpans(context.Background(), tc.request)
			if tc.expectedErr != nil {
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NotNil(t, resp)
				assert.NoError(t, err)
			}

			expectMetrics(t, reader, tc.expectedMetrics)
		})
	}
}

func newPostSpansRequest(t *testing.T) *api_v2.PostSpansRequest {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	spans := resourceSpans.ScopeSpans().AppendEmpty()
	span0 := spans.Spans().AppendEmpty()
	span0.SetTraceID(pcommon.TraceID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	span0.SetSpanID(pcommon.SpanID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	span1 := spans.Spans().AppendEmpty()
	span1.SetTraceID(pcommon.TraceID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	span1.SetSpanID(pcommon.SpanID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	batches, err := jaegertranslator.ProtoFromTraces(traces)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	return &api_v2.PostSpansRequest{Batch: *batches[0]}
}

func TestGRPCSampler_GetSamplingStrategy(t *testing.T) {
	type testcase struct {
		params               *api_v2.SamplingStrategyParameters
		fetcher              agentcfg.Fetcher
		expectedSamplingRate float64
		expectedErrMsg       string
		expectedLogMsg       string
		expectedLogError     string
	}

	for name, tc := range map[string]testcase{
		"unauthorized": {
			params:           &api_v2.SamplingStrategyParameters{ServiceName: unauthorizedServiceName},
			expectedErrMsg:   "no sampling rate available",
			expectedLogMsg:   "no valid sampling rate fetched",
			expectedLogError: `unauthorized: anonymous access not permitted for service "serviceB"`,
		},
		"withSamplingRate": {
			params: &api_v2.SamplingStrategyParameters{ServiceName: authorizedServiceName},
			fetcher: mockAgentConfigFetcher(agentcfg.Result{
				Source: agentcfg.Source{
					Settings: agentcfg.Settings{
						agentcfg.TransactionSamplingRateKey: "0.75",
					},
				},
			}, nil),
			expectedSamplingRate: 0.75,
		},
		"noSamplingRate": {
			params: &api_v2.SamplingStrategyParameters{ServiceName: authorizedServiceName},
			fetcher: mockAgentConfigFetcher(agentcfg.Result{
				Source: agentcfg.Source{
					Settings: agentcfg.Settings{},
				},
			}, nil),
			expectedErrMsg: "no sampling rate available",
			expectedLogMsg: "no valid sampling rate fetched",
		},
		"invalidSamplingRate": {
			params: &api_v2.SamplingStrategyParameters{ServiceName: authorizedServiceName},
			fetcher: mockAgentConfigFetcher(agentcfg.Result{
				Source: agentcfg.Source{
					Settings: agentcfg.Settings{
						agentcfg.TransactionSamplingRateKey: "foo",
					},
				},
			}, nil),
			expectedErrMsg: "no sampling rate available",
			expectedLogMsg: "no valid sampling rate fetched",
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))

			conn, logs := newServer(t, nil, tc.fetcher)
			client := api_v2.NewSamplingManagerClient(conn)
			resp, err := client.GetSamplingStrategy(context.Background(), tc.params)

			// assert sampling response
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				assert.Nil(t, resp)

				require.Equal(t, 1, logs.Len())
				log := logs.All()[0]
				assert.Contains(t, log.Message, tc.expectedLogMsg)
				if tc.expectedLogError != "" {
					assert.Equal(t, tc.expectedLogError, log.ContextMap()["error"])
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, api_v2.SamplingStrategyType_PROBABILISTIC, resp.StrategyType)
				assert.Equal(t, tc.expectedSamplingRate, resp.ProbabilisticSampling.SamplingRate)
				assert.Nil(t, resp.OperationSampling)
				assert.Nil(t, resp.RateLimitingSampling)
			}
		})
	}
}

const (
	authorizedServiceName   = "serviceA"
	unauthorizedServiceName = "serviceB"
)

func newServer(t *testing.T, batchProcessor modelpb.BatchProcessor, agentcfgFetcher agentcfg.Fetcher) (*grpc.ClientConn, *observer.ObservedLogs) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	authenticator, err := auth.NewAuthenticator(config.AgentAuth{
		Anonymous: config.AnonymousAgentAuth{
			Enabled:      true,
			AllowService: []string{authorizedServiceName},
		},
		SecretToken: "abc123",
	})
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptors.Auth(authenticator),
		interceptors.NewMetricsUnaryServerInterceptor(map[string]string{}),
	))
	semaphore := semaphore.NewWeighted(1)

	core, observedLogs := observer.New(zap.DebugLevel)
	RegisterGRPCServices(srv, zap.New(core), batchProcessor, agentcfgFetcher, semaphore)

	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn, observedLogs
}

func mockAgentConfigFetcher(result agentcfg.Result, err error) agentcfg.Fetcher {
	return fetchAgentConfigFunc(func(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
		return result, err
	})
}

type fetchAgentConfigFunc func(context.Context, agentcfg.Query) (agentcfg.Result, error)

func (f fetchAgentConfigFunc) Fetch(ctx context.Context, query agentcfg.Query) (agentcfg.Result, error) {
	return f(ctx, query)
}

func expectMetrics(t *testing.T, reader sdkmetric.Reader, expectedMetrics map[string]int64) {
	t.Helper()

	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))
	foundMetrics := []string{}
	for _, sm := range rm.ScopeMetrics {

		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)

				if v, ok := expectedMetrics[m.Name]; ok {
					assert.Equal(t, v, d.DataPoints[0].Value, m.Name)
				} else {
					assert.Fail(t, "unexpected metric", m.Name)
				}
			}
		}
	}

	assert.Equal(t, len(expectedMetrics), len(foundMetrics))
}
