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

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/kibana/kibanatest"
)

func TestGRPCCollector_PostSpans(t *testing.T) {
	for name, tc := range map[string]testGRPCCollector{
		"empty request": {
			request: &api_v2.PostSpansRequest{},
		},
		"successful request": {},
		"failing request": {
			consumerErr: errors.New("consumer failed"),
			expectedErr: errors.New("consumer failed"),
		},
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
		})
	}
}

type testGRPCCollector struct {
	request     *api_v2.PostSpansRequest
	consumer    tracesConsumerFunc
	consumerErr error
	collector   *grpcCollector

	expectedErr error
}

func (tc *testGRPCCollector) setup(t *testing.T) {
	beatertest.ClearRegistry(gRPCCollectorMonitoringMap)
	if tc.request == nil {
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
		tc.request = &api_v2.PostSpansRequest{Batch: *batches[0]}
	}

	if tc.consumer == nil {
		tc.consumer = func(ctx context.Context, td pdata.Traces) error {
			return tc.consumerErr
		}
	}
	tc.collector = &grpcCollector{tc.consumer}
}

type tracesConsumerFunc func(ctx context.Context, td pdata.Traces) error

func (f tracesConsumerFunc) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (f tracesConsumerFunc) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return f(ctx, td)
}

func nopConsumer() tracesConsumerFunc {
	return func(context.Context, pdata.Traces) error { return nil }
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
			expectedErrMsg: "no sampling rate available",
			expectedLogMsg: "No valid sampling rate fetched",
		},
		"invalidSamplingRate": {
			kibanaBody: map[string]interface{}{
				"_id": "1",
				"_source": map[string]interface{}{
					"settings": map[string]interface{}{
						agentcfg.TransactionSamplingRateKey: "foo"}}},
			expectedErrMsg: "no sampling rate available",
			expectedLogMsg: "No valid sampling rate fetched",
		},
		"unsupportedVersion": {
			kibanaVersion:  common.MustNewVersion("7.4.0"),
			expectedErrMsg: "agent remote configuration not supported",
			expectedLogMsg: "Kibana client does not support",
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
			tc.setup()
			params := &api_v2.SamplingStrategyParameters{ServiceName: "serviceA"}

			authenticator, err := auth.NewAuthenticator(config.AgentAuth{
				Anonymous: config.AnonymousAgentAuth{Enabled: true},
			})
			require.NoError(t, err)
			ctx := context.Background()
			_, authz, err := authenticator.Authenticate(ctx, "", "")
			require.NoError(t, err)
			ctx = auth.ContextWithAuthorizer(ctx, authz)
			resp, err := tc.sampler.GetSamplingStrategy(ctx, params)

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
				assert.Contains(t, logs, tc.expectedLogMsg)
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

type testGRPCSampler struct {
	kibanaBody    map[string]interface{}
	kibanaCode    int
	kibanaVersion *common.Version
	sampler       *grpcSampler

	expectedErrMsg       string
	expectedLogMsg       string
	expectedSamplingRate float64
}

func (tc *testGRPCSampler) setup() {
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
	client := kibanatest.MockKibana(tc.kibanaCode, tc.kibanaBody, *tc.kibanaVersion, true)
	fetcher := agentcfg.NewKibanaFetcher(client, time.Second)
	tc.sampler = &grpcSampler{logp.L(), fetcher}
	beatertest.ClearRegistry(gRPCSamplingMonitoringMap)
}
