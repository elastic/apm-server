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
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/request"
)

var usedResults = []request.ResultID{
	request.IDRequestCount,
	request.IDResponseCount,
	request.IDResponseErrorsCount,
	request.IDResponseValidCount,
	request.IDResponseErrorsUnauthorized,
	request.IDResponseErrorsTimeout,
	request.IDResponseErrorsRateLimit,
}

// NewMetricsUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that increments metrics
// for gRPC method calls.
func NewMetricsUnaryServerInterceptor(metricPrefix map[string]string) grpc.UnaryServerInterceptor {
	return newMetricsUnaryServerInterceptor(metricPrefix).Intercept()
}

// metricsPrefixOverride allows each handler to set their own metrics prefix,
// and override the one set globally.
type metricsPrefixOverride interface {
	// MetricsPrefix is the metrics prefix override.
	// It takes an argument, which is `info.FullMethod`, and returns a string, which is the new prefix.
	MetricsPrefix(string) string
}

type metricsUnaryServerInterceptor struct {
	prefixes map[string]string
	meter    metric.Meter
	counters map[string]metric.Int64Counter
}

func newMetricsUnaryServerInterceptor(p map[string]string) *metricsUnaryServerInterceptor {
	return &metricsUnaryServerInterceptor{
		prefixes: p,
		meter:    otel.Meter("internal/beater/middleware"),
		counters: map[string]metric.Int64Counter{},
	}
}

func (i *metricsUnaryServerInterceptor) Intercept() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		prefix := i.prefixes[info.FullMethod]
		if srv, ok := info.Server.(metricsPrefixOverride); ok {
			prefix = srv.MetricsPrefix(info.FullMethod)
		}

		i.getMetric(prefix, request.IDRequestCount).Add(ctx, 1)

		defer i.getMetric(prefix, request.IDResponseCount).Add(ctx, 1)

		resp, err := handler(ctx, req)

		responseID := request.IDResponseValidCount
		if err != nil {
			responseID = request.IDResponseErrorsCount
			if s, ok := status.FromError(err); ok {
				switch s.Code() {
				case codes.Unauthenticated:
					i.getMetric(prefix, request.IDResponseErrorsUnauthorized).Add(ctx, 1)
				case codes.DeadlineExceeded, codes.Canceled:
					i.getMetric(prefix, request.IDResponseErrorsTimeout).Add(ctx, 1)
				case codes.ResourceExhausted:
					i.getMetric(prefix, request.IDResponseErrorsRateLimit).Add(ctx, 1)
				}
			}
		}

		i.getMetric(prefix, responseID).Add(ctx, 1)

		return resp, err
	}
}

func (i *metricsUnaryServerInterceptor) getMetric(prefix string, n request.ResultID) metric.Int64Counter {
	name := strings.Join([]string{prefix, string(n)}, ".")

	if m, ok := i.counters[name]; ok {
		return m
	}

	nm, _ := i.meter.Int64Counter(string(name))
	i.counters[name] = nm
	return nm
}
