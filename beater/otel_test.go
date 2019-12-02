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

package beater

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"
	"testing"

	"google.golang.org/grpc/credentials"

	"github.com/elastic/beats/libbeat/common/transport/tlscommon"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm"
	"google.golang.org/grpc"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func Test_otelCollector(t *testing.T) {
	setupOtelCollector := func(t *testing.T, cfg *config.Config) *otelCollector {
		logger := logp.NewLogger("otelTest")
		tracer := apm.DefaultTracer
		reporter := func(context.Context, publish.PendingReq) error { return nil }
		otelColl, err := newOtelCollector(logger, cfg, tracer, reporter)
		require.NoError(t, err)
		return otelColl
	}

	t.Run("new", func(t *testing.T) {
		cfg := config.DefaultConfig("9.9.9")
		cfg.OtelConfig.Jaeger.Enabled = true
		c := setupOtelCollector(t, cfg)
		assert.Equal(t, 1, len(c.collectors))
		assert.NotNil(t, c.observer)

	})

	t.Run("start", func(t *testing.T) {
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
		c := setupOtelCollector(t, config.DefaultConfig("9.9.9"))
		testColl := &testCollector{}
		c.collectors = []collector{testColl}

		var wg sync.WaitGroup

		// return with error on observer error
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c.start()
			require.Error(t, err)
			assert.Equal(t, "stop test", err.Error())
		}()
		c.observer.fatal(errors.New("stop test"))
		wg.Wait()

		require.True(t, testColl.started)
		logs := logp.ObserverLogs().TakeAll()
		assert.Equal(t, 1, len(logs))
		assert.Contains(t, logs[0].Message, "Starting collector")

		// return with error on ctx.Done()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c.start()
			require.Error(t, err)
			assert.Equal(t, "context canceled", err.Error())
		}()
		c.observer.cancel()
		wg.Wait()
	})

	t.Run("stop", func(t *testing.T) {
		require.NoError(t, logp.DevelopmentSetup(logp.ToObserverOutput()))
		c := setupOtelCollector(t, config.DefaultConfig("9.9.9"))
		testColl := &testCollector{}
		c.collectors = []collector{testColl}
		c.stop()

		logs := logp.ObserverLogs().TakeAll()
		assert.Equal(t, 1, len(logs))
		assert.Contains(t, logs[0].Message, "Stopping collector")

		assert.True(t, testColl.stopped)
	})
}

type testCollector struct {
	started, stopped bool
}

func (tc *testCollector) start() error     { tc.started = true; return nil }
func (tc *testCollector) stop()            { tc.stopped = true }
func (tc *testCollector) name() string     { return "testCollector" }
func (tc *testCollector) endpoint() string { return "testCollector:12345" }

func Test_jaegerCollector(t *testing.T) {
	for name, tc := range map[string]testcaseJaeger{
		"no otel config": {cfg: &config.Config{}},
		"default config": {cfg: config.DefaultConfig("9.9.9")},
		"withJaegerDefault": {
			cfg: func() *config.Config {
				cfg := config.DefaultConfig("8.0.0")
				cfg.OtelConfig.Jaeger.Enabled = true
				return cfg
			}()},
		"withJaeger withTLS": {
			cfg: &config.Config{OtelConfig: &config.OtelConfig{
				Jaeger: config.JaegerConfig{
					Enabled: true,
					GRPC: config.GRPCConfig{
						Host: "localhost:4444",
						TLS: &tlscommon.CertificateConfig{
							Certificate: path.Join("..", "testdata", "tls", "certificate.pem"),
							Key:         path.Join("..", "testdata", "tls", "key.pem"),
						},
					}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			tc.setup(t)

			if tc.cfg.OtelConfig == nil || !tc.cfg.OtelConfig.Jaeger.Enabled {
				assert.Nil(t, tc.collector)
			} else {
				// start
				require.NoError(t, tc.collector.start())

				// send data via grpc client
				span := &tracepb.Span{
					TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
					SpanId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}
				td := consumerdata.TraceData{Spans: []*tracepb.Span{span}}
				batch, err := jaeger.OCProtoToJaegerProto(td)
				require.NoError(t, err)
				require.NotNil(t, batch)
				jaegerCollectorClient := api_v2.NewCollectorServiceClient(tc.client)
				_, err = jaegerCollectorClient.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *batch})
				require.NoError(t, err)
				assert.Equal(t, 1, len(tc.transformed))

				// stop grpc server
				tc.collector.stop()
				_, err = jaegerCollectorClient.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *batch})
				assert.Error(t, err)

				assert.Equal(t, tc.cfg.OtelConfig.Jaeger.GRPC.Host, tc.collector.endpoint())
				assert.Equal(t, "jaeger", tc.collector.name())

			}

		})
	}

}

type testcaseJaeger struct {
	cfg         *config.Config
	reporter    func(ctx context.Context, pub publish.PendingReq) error
	transformed []transform.Transformable
	collector   *jaegerCollector
	client      *grpc.ClientConn
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
	traceConsumer := &Consumer{Reporter: tc.reporter}
	var err error
	tc.collector, err = newJaegerCollector(tc.cfg, traceConsumer, apm.DefaultTracer, newObserver(context.Background()))
	require.NoError(t, err)

	if tc.cfg.OtelConfig == nil || !tc.cfg.OtelConfig.Jaeger.Enabled {
		return
	}

	// build grpc client
	if tc.cfg.OtelConfig.Jaeger.GRPC.TLS != nil {
		creds, err := credentials.NewClientTLSFromFile(tc.cfg.OtelConfig.Jaeger.GRPC.TLS.Certificate, "apm-server")
		require.NoError(t, err)
		tc.client, err = grpc.Dial(tc.cfg.OtelConfig.Jaeger.GRPC.Host, grpc.WithTransportCredentials(creds))
		require.NoError(t, err)

	} else {
		fmt.Println("---------------- Insecure client")
		tc.client, err = grpc.Dial(tc.cfg.OtelConfig.Jaeger.GRPC.Host, grpc.WithInsecure())
		require.NoError(t, err)
	}
}
