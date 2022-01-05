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
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/kibana/kibanatest"
	"github.com/elastic/apm-server/model"
)

func TestPostSpans(t *testing.T) {
	var processorErr error
	var processor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
		return processorErr
	}
	conn := newServer(t, processor, nil)

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
	traces := pdata.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	spans := resourceSpans.InstrumentationLibrarySpans().AppendEmpty()
	span0 := spans.Spans().AppendEmpty()
	span0.SetTraceID(pdata.NewTraceID([16]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}))
	span0.SetSpanID(pdata.NewSpanID([8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}))
	span1 := spans.Spans().AppendEmpty()
	span1.SetTraceID(pdata.NewTraceID([16]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}))
	span1.SetSpanID(pdata.NewSpanID([8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}))

	batches, err := jaegertranslator.InternalTracesToJaegerProto(traces)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	return &api_v2.PostSpansRequest{Batch: *batches[0]}
}

func TestApprovals(t *testing.T) {
	for _, name := range []string{"batch_0", "batch_1"} {
		t.Run(name, func(t *testing.T) {
			var batches []model.Batch
			var processor model.ProcessBatchFunc = func(ctx context.Context, batch *model.Batch) error {
				batches = append(batches, *batch)
				return nil
			}
			conn := newServer(t, processor, nil)
			client := api_v2.NewCollectorServiceClient(conn)

			f := filepath.Join("..", "..", "testdata", "jaeger", name)
			data, err := ioutil.ReadFile(f + ".json")
			require.NoError(t, err)

			var request api_v2.PostSpansRequest
			err = json.Unmarshal(data, &request)
			require.NoError(t, err)
			_, err = client.PostSpans(context.Background(), &request)
			require.NoError(t, err)

			require.Len(t, batches, 1)
			events := batches[0].Transform(context.Background())
			docs := beatertest.EncodeEventDocs(events...)
			approvaltest.ApproveEventDocs(t, f, docs)
		})
	}
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
			expectedLogMsg:   "No valid sampling rate fetched",
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
			expectedLogMsg: "No valid sampling rate fetched",
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
			expectedLogMsg: "No valid sampling rate fetched",
		},
		"unsupportedVersion": {
			// Trigger the agentcfg.ValidationError code path by using agentcfg.KibanaFetcher
			// with an invalid (too old) Kibana version.
			params: &api_v2.SamplingStrategyParameters{ServiceName: authorizedServiceName},
			fetcher: agentcfg.NewKibanaFetcher(
				kibanatest.MockKibana(200, nil, *common.MustNewVersion("7.4.0"), true),
				time.Second,
			),
			expectedErrMsg: "agent remote configuration not supported",
			expectedLogMsg: "Kibana client does not support",
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))

			conn := newServer(t, nil, tc.fetcher)
			client := api_v2.NewSamplingManagerClient(conn)
			resp, err := client.GetSamplingStrategy(context.Background(), tc.params)

			// assert sampling response
			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				assert.Nil(t, resp)

				logs := logp.ObserverLogs()
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

func newServer(t *testing.T, batchProcessor model.BatchProcessor, agentcfgFetcher agentcfg.Fetcher) *grpc.ClientConn {
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
	srv := grpc.NewServer(grpc.UnaryInterceptor(interceptors.Auth(MethodAuthenticators(authenticator))))

	logger := logp.NewLogger("jaeger.test")
	RegisterGRPCServices(srv, logger, batchProcessor, agentcfgFetcher)

	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
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
