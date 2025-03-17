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

package interceptors

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp"
)

func TestMetrics(t *testing.T) {
	for _, metrics := range []struct {
		methodName string
		prefix     string
	}{
		{
			methodName: "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
			prefix:     "apm-server.otlp.grpc.metrics.",
		},
		{
			methodName: "/opentelemetry.proto.collector.trace.v1.TraceService/Export",
			prefix:     "apm-server.otlp.grpc.traces.",
		},
		{
			methodName: "/opentelemetry.proto.collector.logs.v1.LogsService/Export",
			prefix:     "apm-server.otlp.grpc.logs.",
		},
	} {
		reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
			func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
				return metricdata.DeltaTemporality
			},
		))
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		logger := logp.NewLogger("interceptor.metrics.test")

		interceptor := Metrics(logger, mp)

		ctx := context.Background()
		info := &grpc.UnaryServerInfo{
			FullMethod: metrics.methodName,
		}

		for _, tc := range []struct {
			name          string
			f             func(ctx context.Context, req interface{}) (interface{}, error)
			monitoringInt map[request.ResultID]int64
			expectedOtel  map[string]interface{}
		}{
			{
				name: "with an error",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, errors.New("error")
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):        1,
					string(request.IDResponseCount):       1,
					string(request.IDResponseErrorsCount): 1,

					"request.duration": 1,
				},
			},
			{
				name: "with an unauthenticated error",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.Unauthenticated, "error")
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):               1,
					string(request.IDResponseCount):              1,
					string(request.IDResponseErrorsCount):        1,
					string(request.IDResponseErrorsUnauthorized): 1,

					"request.duration": 1,
				},
			},
			{
				name: "with a deadline exceeded error",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.DeadlineExceeded, "request timed out")
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):          1,
					string(request.IDResponseCount):         1,
					string(request.IDResponseErrorsCount):   1,
					string(request.IDResponseErrorsTimeout): 1,

					"request.duration": 1,
				},
			},
			{
				name: "with a canceled error",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.Canceled, "request timed out")
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):          1,
					string(request.IDResponseCount):         1,
					string(request.IDResponseErrorsCount):   1,
					string(request.IDResponseErrorsTimeout): 1,

					"request.duration": 1,
				},
			},
			{
				name: "with a resource exhausted error",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):            1,
					string(request.IDResponseCount):           1,
					string(request.IDResponseErrorsCount):     1,
					string(request.IDResponseErrorsRateLimit): 1,

					"request.duration": 1,
				},
			},
			{
				name: "with a success",
				f: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, nil
				},
				expectedOtel: map[string]interface{}{
					string(request.IDRequestCount):       1,
					string(request.IDResponseCount):      1,
					string(request.IDResponseValidCount): 1,

					"request.duration": 1,
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				interceptor(ctx, nil, info, tc.f)

				expectedMetrics := make(map[string]any, 2*len(tc.expectedOtel))

				for k, v := range tc.expectedOtel {
					// add otel metrics
					expectedMetrics["grpc.server."+k] = v

					if k != "request.duration" {
						// add legacy metrics
						expectedMetrics[metrics.prefix+k] = v
					}
				}

				monitoringtest.ExpectOtelMetrics(t, reader, expectedMetrics)
			})
		}
	}
}

func TestMetrics_ConcurrentSafe(t *testing.T) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	logger := logp.NewLogger("interceptor.metrics.test")
	interceptor := Metrics(logger, mp)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/opentelemetry.proto.collector.trace.v1.TraceService/Export",
	}

	doNothing := func(ctx context.Context, req interface{}) (interface{}, error) {
		return req, nil
	}

	type respAndErr struct {
		resp interface{}
		err  error
	}

	const numG = 10
	ch := make(chan respAndErr, numG)
	var wg sync.WaitGroup
	for range numG {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := interceptor(ctx, "hello", info, doNothing)
			ch <- respAndErr{resp: resp, err: err}
		}()
	}

	wg.Wait()
	close(ch)
	for r := range ch {
		assert.Equal(t, "hello", r.resp, "unexpected response")
		assert.NoError(t, r.err, "unexpected error")
	}

	monitoringtest.ExpectContainOtelMetrics(t, reader, map[string]any{
		"grpc.server.request.count": numG,
	})
}
