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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/request"
)

func TestMetricsUnaryServerInterceptor(t *testing.T) {
	ctx := context.Background()
	methodName := "test_method_name"

	for _, tt := range []struct {
		name          string
		defaultPrefix string
		info          *grpc.UnaryServerInfo

		expectedPrefix string
	}{
		{
			name:          "with the default prefix",
			defaultPrefix: "prefix",
			info: &grpc.UnaryServerInfo{
				FullMethod: methodName,
			},
			expectedPrefix: "prefix",
		},
		{
			name:          "with a custom prefix",
			defaultPrefix: "prefix",
			info: &grpc.UnaryServerInfo{
				FullMethod: methodName,
				Server: metricsOverrideFn(func(m string) string {
					return methodName
				}),
			},

			expectedPrefix: "new_prefix",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewMetricsUnaryServerInterceptor(map[string]string{methodName: tt.defaultPrefix})

			for _, tc := range []struct {
				name     string
				f        func(ctx context.Context, req interface{}) (interface{}, error)
				expected map[request.ResultID]int64
			}{
				{
					name: "with an error",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, errors.New("error")
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:        1,
						request.IDResponseCount:       1,
						request.IDResponseErrorsCount: 1,
					},
				},
				{
					name: "with an unauthenticated error",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, status.Error(codes.Unauthenticated, "error")
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:               1,
						request.IDResponseCount:              1,
						request.IDResponseErrorsCount:        1,
						request.IDResponseErrorsUnauthorized: 1,
					},
				},
				{
					name: "with a deadline exceeded",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, status.Error(codes.DeadlineExceeded, "request timed out")
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:          1,
						request.IDResponseCount:         1,
						request.IDResponseErrorsCount:   1,
						request.IDResponseErrorsTimeout: 1,
					},
				},
				{
					name: "with a canceled",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, status.Error(codes.Canceled, "request timed out")
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:          1,
						request.IDResponseCount:         1,
						request.IDResponseErrorsCount:   1,
						request.IDResponseErrorsTimeout: 1,
					},
				},
				{
					name: "with a resources exhausted",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:            1,
						request.IDResponseCount:           1,
						request.IDResponseErrorsCount:     1,
						request.IDResponseErrorsRateLimit: 1,
					},
				},
				{
					name: "with no error",
					f: func(ctx context.Context, req interface{}) (interface{}, error) {
						return nil, nil
					},
					expected: map[request.ResultID]int64{
						request.IDRequestCount:       1,
						request.IDResponseCount:      1,
						request.IDResponseValidCount: 1,
					},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
						func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
							return metricdata.DeltaTemporality
						},
					))
					otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

					interceptor(ctx, nil, tt.info, tc.f)

					var rm metricdata.ResourceMetrics
					assert.NoError(t, reader.Collect(context.Background(), &rm))

					for _, sm := range rm.ScopeMetrics {
						assert.Equal(t, len(tc.expected), len(sm.Metrics))

						for _, m := range sm.Metrics {
							switch d := m.Data.(type) {
							case metricdata.Sum[int64]:
								assert.Equal(t, 1, len(d.DataPoints))
								b, name, _ := strings.Cut(m.Name, tt.expectedPrefix+".")

								assert.Equalf(t, "", b, "expected metrics prefix to be %s, was %s", tt.expectedPrefix, b)
								if v, ok := tc.expected[request.ResultID(name)]; ok {
									assert.Equal(t, v, d.DataPoints[0].Value)
								} else {
									assert.Fail(t, "unexpected metric", m.Name)
								}
							}
						}
					}
				})
			}
		})
	}
}

type metricsOverrideFn func(string) string

func (f metricsOverrideFn) MetricsPrefix(m string) string {
	return f(m)
}
