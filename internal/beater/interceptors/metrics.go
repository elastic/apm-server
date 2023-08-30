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
	meter := otel.Meter("internal/beater/middleware")
	metrics := map[request.ResultID]map[string]metric.Int64Counter{}

	for _, v := range usedResults {
		for m, p := range metricPrefix {
			nm, err := meter.Int64Counter(strings.Join([]string{p, string(v)}, "."))
			if err == nil {
				if _, ok := metrics[v]; !ok {
					metrics[v] = map[string]metric.Int64Counter{}
				}

				metrics[v][m] = nm
			}
		}
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		addMetric(ctx, metrics, request.IDRequestCount, info.FullMethod, 1)

		defer addMetric(ctx, metrics, request.IDResponseCount, info.FullMethod, 1)

		resp, err := handler(ctx, req)

		responseID := request.IDResponseValidCount
		if err != nil {
			responseID = request.IDResponseErrorsCount
			if s, ok := status.FromError(err); ok {
				switch s.Code() {
				case codes.Unauthenticated:
					addMetric(ctx, metrics, request.IDResponseErrorsUnauthorized, info.FullMethod, 1)
				case codes.DeadlineExceeded, codes.Canceled:
					addMetric(ctx, metrics, request.IDResponseErrorsTimeout, info.FullMethod, 1)
				case codes.ResourceExhausted:
					addMetric(ctx, metrics, request.IDResponseErrorsRateLimit, info.FullMethod, 1)
				}
			}
		}

		addMetric(ctx, metrics, responseID, info.FullMethod, 1)

		return resp, err
	}
}

func addMetric(ctx context.Context, metrics map[request.ResultID]map[string]metric.Int64Counter, result request.ResultID, method string, c int64) {
	if r, ok := metrics[result]; ok {
		if m, ok := r[method]; ok {
			m.Add(ctx, c)
		}
	}
}
