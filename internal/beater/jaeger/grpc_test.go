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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func TestPostSpans(t *testing.T) {
	var processorErr error
	var processor modelpb.ProcessBatchFunc = func(ctx context.Context, batch *modelpb.Batch) error {
		return processorErr
	}
	conn, logs := newServer(t, processor, nil)

	client := api_v2.NewCollectorServiceClient(conn)
	result, err := client.PostSpans(context.Background(), &api_v2.PostSpansRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)

	type testcase struct {
		request      *api_v2.PostSpansRequest
		processorErr error
		expectedErr  error
	}

	for name, tc := range map[string]testcase{
		"empty request": {
			request: &api_v2.PostSpansRequest{},
		},
		"successful request": {
			request: newPostSpansRequest(t),
		},
		"failing request": {
			request:      newPostSpansRequest(t),
			processorErr: errors.New("processor failed"),
			expectedErr:  status.Error(codes.Unknown, "processor failed"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			processorErr = tc.processorErr
			resp, err := client.PostSpans(context.Background(), tc.request)

			// Deprecation log shown on server startup.
			log := logs.All()[0]
			assert.Equal(t, log.Level, zap.InfoLevel)
			assert.Contains(t, log.Message, deprecationNotice)

			// Deprecation log shown on first endpoint hit.
			log = logs.All()[1]
			assert.Equal(t, log.Level, zap.WarnLevel)
			assert.Contains(t, log.Message, deprecationNotice)

			if tc.expectedErr != nil {
				assert.Nil(t, resp)
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NotNil(t, resp)
				assert.NoError(t, err)
			}
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

	batches := jaegertranslator.ProtoFromTraces(traces)
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
			conn, logs := newServer(t, nil, tc.fetcher)
			client := api_v2.NewSamplingManagerClient(conn)
			resp, err := client.GetSamplingStrategy(context.Background(), tc.params)

			// Deprecation log shown on server startup.
			log := logs.All()[0]
			assert.Equal(t, log.Level, zap.InfoLevel)
			assert.Contains(t, log.Message, deprecationNotice)

			// Deprecation log shown on first endpoint hit.
			log = logs.All()[1]
			assert.Equal(t, log.Level, zap.WarnLevel)
			assert.Contains(t, log.Message, deprecationNotice)

			// assert sampling response
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				assert.Nil(t, resp)

				require.Equal(t, 3, logs.Len())

				log = logs.All()[2]
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
	}, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(interceptors.Auth(authenticator)))
	semaphore := semaphore.NewWeighted(1)

	core, observedLogs := observer.New(zap.DebugLevel)
	RegisterGRPCServices(srv, zap.New(core), batchProcessor, agentcfgFetcher, semaphore)

	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
