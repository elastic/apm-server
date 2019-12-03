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
	"path"
	"testing"
	"time"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	jaegermodel "github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	oteljaeger "github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func TestGRPCServerIntegration(t *testing.T) {
	enabledTrue, enabledFalse := true, false
	for name, tc := range map[string]testcaseJaeger{
		"default config": {cfg: config.DefaultConfig("9.9.9")},
		"with jaeger": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig("8.0.0")
				cfg.JaegerConfig.Enabled = true
				return cfg
			}()},
		"with jaeger TLS disabled": {
			cfg: &config.Config{
				TLS: &tlscommon.ServerConfig{
					Enabled: &enabledFalse,
					Certificate: tlscommon.CertificateConfig{
						Certificate: path.Join("..", "testdata", "tls", "certificate.pem"),
						Key:         path.Join("..", "testdata", "tls", "key.pem")},
				},
				JaegerConfig: config.JaegerConfig{
					Enabled: true,
					GRPC: config.GRPCConfig{
						Host: "localhost:4444",
					}},
			}},
		"with jaeger with TLS": {
			cfg: &config.Config{
				TLS: &tlscommon.ServerConfig{
					Enabled: &enabledTrue,
					Certificate: tlscommon.CertificateConfig{
						Certificate: path.Join("..", "testdata", "tls", "certificate.pem"),
						Key:         path.Join("..", "testdata", "tls", "key.pem")},
				},
				JaegerConfig: config.JaegerConfig{
					Enabled: true,
					GRPC: config.GRPCConfig{
						Host: "localhost:4444",
					}},
			}},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)

			if !tc.cfg.JaegerConfig.Enabled {
				assert.Nil(t, tc.server)
			} else {
				require.NotNil(t, tc.server)
				require.NotNil(t, tc.client)

				// start
				go func() {
					err := tc.server.Start()
					require.NoError(t, err)
				}()

				err := tc.sendSpans(time.Second)
				require.NoError(t, err)
				assert.Equal(t, 1, len(tc.transformed))

				// stop grpc server
				tc.server.Stop()
				_, err = tc.client.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *tc.batch})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "connection refused")
			}
		})
	}

}

type testcaseJaeger struct {
	cfg         *config.Config
	reporter    func(ctx context.Context, pub publish.PendingReq) error
	transformed []transform.Transformable
	server      *GRPCServer
	client      api_v2.CollectorServiceClient
	batch       *jaegermodel.Batch
}

func (tc *testcaseJaeger) setup(t *testing.T) {
	if tc.cfg == nil {
		tc.cfg = config.DefaultConfig("9.9.9")
	}

	// build grpc receiver
	tc.reporter = func(ctx context.Context, pub publish.PendingReq) error {
		tc.transformed = append(tc.transformed, pub.Transformables...)
		return nil
	}
	var err error
	tc.server, err = NewGRPCServer(logp.NewLogger("jaeger"), tc.cfg, apm.DefaultTracer, tc.reporter)
	require.NoError(t, err)

	if !tc.cfg.JaegerConfig.Enabled {
		return
	}

	// build grpc client and batch for request
	var client *grpc.ClientConn
	if tc.cfg.JaegerConfig.GRPC.TLS != nil {
		client, err = grpc.Dial(tc.cfg.JaegerConfig.GRPC.Host,
			grpc.WithTransportCredentials(credentials.NewTLS(tc.cfg.JaegerConfig.GRPC.TLS)))
		require.NoError(t, err)

	} else {
		client, err = grpc.Dial(tc.cfg.JaegerConfig.GRPC.Host, grpc.WithInsecure())
		require.NoError(t, err)
	}
	tc.client = api_v2.NewCollectorServiceClient(client)

	// send data via grpc client
	td := consumerdata.TraceData{Spans: []*v1.Span{{
		TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		SpanId:  []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}}}
	tc.batch, err = oteljaeger.OCProtoToJaegerProto(td)
	require.NoError(t, err)
	require.NotNil(t, tc.batch)
}

func (tc *testcaseJaeger) sendSpans(timeout time.Duration) error {
	start := time.Now()
	var err error
	for {
		_, err = tc.client.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *tc.batch})
		if err == nil {
			break
		}
		if time.Since(start) > timeout {
			err = errors.New("timeout")
			break
		}
		time.Sleep(time.Second / 50)
	}
	return err
}
