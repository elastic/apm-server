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
	"sync"
	"testing"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm"
	"google.golang.org/grpc"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

func Test_otelCollector(t *testing.T) {

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

func Test_jaegerCollector(t *testing.T) {
	// jaeger not enabled
	c, err := newJaegerCollector(&config.Config{}, nil, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, c)

	// default config
	cfg := config.DefaultConfig("9.9.9")
	c, err = newJaegerCollector(cfg, nil, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, c)

	// setup jaeger collector
	cfg.OtelConfig.Jaeger.Enabled = true
	endpoint := "localhost:44444"
	cfg.OtelConfig.Jaeger.GRPC.Host = endpoint
	var transformed []transform.Transformable
	testReporter := func(ctx context.Context, pub publish.PendingReq) error {
		transformed = append(transformed, pub.Transformables...)
		return nil
	}
	traceConsumer := &Consumer{Reporter: testReporter}
	c, err = newJaegerCollector(cfg, traceConsumer, apm.DefaultTracer, newObserver(context.Background()))
	require.NoError(t, err)
	require.NotNil(t, c)

	assert.Equal(t, endpoint, c.endpoint())
	assert.Equal(t, "jaeger", c.name())

	// start
	require.NoError(t, c.start())

	// build grpc client and send data
	client := grpcClient(t, c.endpoint())
	jaegerCollectorClient := api_v2.NewCollectorServiceClient(client)
	span := &tracepb.Span{
		TraceId: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		SpanId:  []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}
	td := consumerdata.TraceData{Spans: []*tracepb.Span{span}}
	batch, err := jaeger.OCProtoToJaegerProto(td)
	require.NoError(t, err)
	require.NotNil(t, batch)

	// send data via grpc client and expect them to be collected
	_, err = jaegerCollectorClient.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *batch})
	require.NoError(t, err)
	assert.Equal(t, 1, len(transformed))

	// stop grpc server
	c.stop()
	_, err = jaegerCollectorClient.PostSpans(context.Background(), &api_v2.PostSpansRequest{Batch: *batch})
	assert.Error(t, err)
}

func setupOtelCollector(t *testing.T, cfg *config.Config) *otelCollector {
	logger := logp.NewLogger("otelTest")
	tracer := apm.DefaultTracer
	reporter := func(context.Context, publish.PendingReq) error { return nil }
	otelColl, err := newOtelCollector(logger, cfg, tracer, reporter)
	require.NoError(t, err)
	return otelColl
}

func grpcClient(t *testing.T, host string) *grpc.ClientConn {
	c, err := grpc.Dial(host, grpc.WithInsecure())
	require.NoError(t, err)
	return c
}

type testCollector struct {
	started, stopped bool
}

func (tc *testCollector) start() error     { tc.started = true; return nil }
func (tc *testCollector) stop()            { tc.stopped = true }
func (tc *testCollector) name() string     { return "testCollector" }
func (tc *testCollector) endpoint() string { return "testCollector:12345" }
