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
<<<<<<< HEAD
=======
	"github.com/stretchr/testify/require"
>>>>>>> 2ca9f908 (fix: metrics interceptor concurrent map read write (#16182))
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var monitoringKeys = append(
	request.DefaultResultIDs,
	request.IDResponseErrorsRateLimit,
	request.IDResponseErrorsTimeout,
	request.IDResponseErrorsUnauthorized,
)

func TestMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	registry := monitoring.NewRegistry()

	monitoringMap := request.MonitoringMapForRegistry(registry, monitoringKeys)
	methodName := "test_method_name"
	logger := logp.NewLogger("interceptor.metrics.test")

	interceptor := Metrics(logger, mp)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{
		FullMethod: methodName,
		Server: requestMetricsFunc(func(fullMethod string) map[request.ResultID]*monitoring.Int {
			assert.Equal(t, methodName, fullMethod)
			return monitoringMap
		}),
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
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         0,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsRateLimit:    0,
				request.IDResponseErrorsTimeout:      0,
				request.IDResponseErrorsUnauthorized: 0,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):        1,
				"grpc.server." + string(request.IDResponseCount):       1,
				"grpc.server." + string(request.IDResponseErrorsCount): 1,

				"grpc.server.request.duration": 1,
			},
		},
		{
			name: "with an unauthenticated error",
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.Unauthenticated, "error")
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         0,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsInternal:     0,
				request.IDResponseErrorsRateLimit:    0,
				request.IDResponseErrorsTimeout:      0,
				request.IDResponseErrorsUnauthorized: 1,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):               1,
				"grpc.server." + string(request.IDResponseCount):              1,
				"grpc.server." + string(request.IDResponseErrorsCount):        1,
				"grpc.server." + string(request.IDResponseErrorsUnauthorized): 1,

				"grpc.server.request.duration": 1,
			},
		},
		{
			name: "with a deadline exceeded error",
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.DeadlineExceeded, "request timed out")
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         0,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsInternal:     0,
				request.IDResponseErrorsRateLimit:    0,
				request.IDResponseErrorsTimeout:      1,
				request.IDResponseErrorsUnauthorized: 0,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):          1,
				"grpc.server." + string(request.IDResponseCount):         1,
				"grpc.server." + string(request.IDResponseErrorsCount):   1,
				"grpc.server." + string(request.IDResponseErrorsTimeout): 1,

				"grpc.server.request.duration": 1,
			},
		},
		{
			name: "with a canceled error",
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.Canceled, "request timed out")
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         0,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsInternal:     0,
				request.IDResponseErrorsRateLimit:    0,
				request.IDResponseErrorsTimeout:      1,
				request.IDResponseErrorsUnauthorized: 0,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):          1,
				"grpc.server." + string(request.IDResponseCount):         1,
				"grpc.server." + string(request.IDResponseErrorsCount):   1,
				"grpc.server." + string(request.IDResponseErrorsTimeout): 1,

				"grpc.server.request.duration": 1,
			},
		},
		{
			name: "with a resource exhausted error",
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         0,
				request.IDResponseErrorsCount:        1,
				request.IDResponseErrorsInternal:     0,
				request.IDResponseErrorsRateLimit:    1,
				request.IDResponseErrorsTimeout:      0,
				request.IDResponseErrorsUnauthorized: 0,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):            1,
				"grpc.server." + string(request.IDResponseCount):           1,
				"grpc.server." + string(request.IDResponseErrorsCount):     1,
				"grpc.server." + string(request.IDResponseErrorsRateLimit): 1,

				"grpc.server.request.duration": 1,
			},
		},
		{
			name: "with a success",
			f: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			},
			monitoringInt: map[request.ResultID]int64{
				request.IDRequestCount:               1,
				request.IDResponseCount:              1,
				request.IDResponseValidCount:         1,
				request.IDResponseErrorsCount:        0,
				request.IDResponseErrorsInternal:     0,
				request.IDResponseErrorsRateLimit:    0,
				request.IDResponseErrorsTimeout:      0,
				request.IDResponseErrorsUnauthorized: 0,
			},
			expectedOtel: map[string]interface{}{
				"grpc.server." + string(request.IDRequestCount):       1,
				"grpc.server." + string(request.IDResponseCount):      1,
				"grpc.server." + string(request.IDResponseValidCount): 1,

				"grpc.server.request.duration": 1,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interceptor(ctx, nil, info, tc.f)
			assertMonitoring(t, tc.monitoringInt, monitoringMap)
			monitoringtest.ClearRegistry(monitoringMap)
			monitoringtest.ExpectOtelMetrics(t, reader, tc.expectedOtel)
		})
	}
}

func assertMonitoring(t *testing.T, expected map[request.ResultID]int64, actual map[request.ResultID]*monitoring.Int) {
	for _, k := range monitoringKeys {
		if val, ok := expected[k]; ok {
			assert.Equalf(t, val, actual[k].Get(), "%s mismatch", k)
		} else {
			assert.Zerof(t, actual[k].Get(), "%s mismatch", k)
		}
	}
}

<<<<<<< HEAD
type requestMetricsFunc func(fullMethod string) map[request.ResultID]*monitoring.Int

func (f requestMetricsFunc) RequestMetrics(fullMethod string) map[request.ResultID]*monitoring.Int {
	return f(fullMethod)
=======
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

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	require.NotEmpty(t, rm.ScopeMetrics)

	findRequestCountMetrics := func() (metricdata.Metrics, bool) {
		for _, mt := range rm.ScopeMetrics[0].Metrics {
			if mt.Name == "grpc.server.request.count" {
				return mt, true
			}
		}
		return metricdata.Metrics{}, false
	}

	mt, exist := findRequestCountMetrics()
	require.True(t, exist)
	counter := mt.Data.(metricdata.Sum[int64])
	assert.EqualValues(t, numG, counter.DataPoints[0].Value, "unexpected counter value")
>>>>>>> 2ca9f908 (fix: metrics interceptor concurrent map read write (#16182))
}
