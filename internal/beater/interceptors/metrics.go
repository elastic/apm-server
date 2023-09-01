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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

var methodUnaryRequestMetrics = make(map[string]map[request.ResultID]*monitoring.Int)

// RegisterMethodUnaryRequestMetrics registers a UnaryRequestMetrics for the
// given full method name. This can be used when the gRPC service implementation
// is not exensible, such as in the case of the OTLP services.
//
// This function must only be called from package init functions; it is not safe
// for concurrent access.
func RegisterMethodUnaryRequestMetrics(fullMethod string, m map[request.ResultID]*monitoring.Int) {
	methodUnaryRequestMetrics[fullMethod] = m
}

// UnaryRequestMetrics is an interface that gRPC services may implement
// to provide a metrics registry for the Metrics interceptor.
type UnaryRequestMetrics interface {
	RequestMetrics(fullMethod string) map[request.ResultID]*monitoring.Int
}

type metricsInterceptor struct {
	logger *logp.Logger
	meter  metric.Meter

	counters map[request.ResultID]metric.Int64Counter
}

func (m *metricsInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var ints map[request.ResultID]*monitoring.Int
		if requestMetrics, ok := info.Server.(UnaryRequestMetrics); ok {
			ints = requestMetrics.RequestMetrics(info.FullMethod)
		} else {
			ints = methodUnaryRequestMetrics[info.FullMethod]
		}
		if ints == nil {
			m.logger.With(
				"grpc.request.method", info.FullMethod,
			).Warn("metrics registry missing")
			return handler(ctx, req)
		}

		m.getMetric(request.IDRequestCount).Add(ctx, 1)
		defer m.getMetric(request.IDResponseCount).Add(ctx, 1)

		ints[request.IDRequestCount].Inc()
		defer ints[request.IDResponseCount].Inc()

		resp, err := handler(ctx, req)

		responseID := request.IDResponseValidCount
		if err != nil {
			responseID = request.IDResponseErrorsCount
			if s, ok := status.FromError(err); ok {
				switch s.Code() {
				case codes.Unauthenticated:
					m.getMetric(request.IDResponseErrorsUnauthorized).Add(ctx, 1)
					ints[request.IDResponseErrorsUnauthorized].Inc()
				case codes.DeadlineExceeded, codes.Canceled:
					m.getMetric(request.IDResponseErrorsTimeout).Add(ctx, 1)
					ints[request.IDResponseErrorsTimeout].Inc()
				case codes.ResourceExhausted:
					m.getMetric(request.IDResponseErrorsRateLimit).Add(ctx, 1)
					ints[request.IDResponseErrorsRateLimit].Inc()
				}
			}
		}

		m.getMetric(responseID).Add(ctx, 1)
		ints[responseID].Inc()

		return resp, err
	}
}

func (m *metricsInterceptor) getMetric(n request.ResultID) metric.Int64Counter {
	name := "grpc." + n
	if met, ok := m.counters[name]; ok {
		return met
	}

	nm, _ := m.meter.Int64Counter(string(name))
	m.counters[name] = nm
	return nm
}

// Metrics returns a grpc.UnaryServerInterceptor that increments metrics
// for gRPC method calls.
//
// If a gRPC service implements UnaryRequestMetrics, its RequestMetrics
// method will be called to obtain the metrics map for incrementing. If the
// service does not implement UnaryRequestMetrics, but
// RegisterMethodUnaryRequestMetrics has been called for the invoked method,
// then the registered UnaryRequestMetrics will be used instead. Finally,
// if neither of these are available, a warning will be logged and no metrics
// will be gathered.
func Metrics(logger *logp.Logger, mp metric.MeterProvider) grpc.UnaryServerInterceptor {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	i := &metricsInterceptor{
		logger: logger,
		meter:  mp.Meter("internal/beater/interceptors"),

		counters: map[request.ResultID]metric.Int64Counter{},
	}

	return i.Interceptor()
}
