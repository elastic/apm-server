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
	"time"

	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	requestDurationHistogram = "request.duration"
)

type metricsInterceptor struct {
	logger *logp.Logger
	meter  metric.Meter

	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Int64Histogram
}

func (m *metricsInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		var legacyMetricsPrefix string

		switch info.FullMethod {
		case "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export":
			legacyMetricsPrefix = "apm-server.otlp.grpc.metrics."
		case "/opentelemetry.proto.collector.trace.v1.TraceService/Export":
			legacyMetricsPrefix = "apm-server.otlp.grpc.traces."
		case "/opentelemetry.proto.collector.logs.v1.LogsService/Export":
			legacyMetricsPrefix = "apm-server.otlp.grpc.logs."
		default:
			m.logger.With(
				"grpc.request.method", info.FullMethod,
			).Warn("metrics registry missing")
			return handler(ctx, req)
		}

		m.inc(ctx, legacyMetricsPrefix, request.IDRequestCount)
		defer m.inc(ctx, legacyMetricsPrefix, request.IDResponseCount)

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		m.getHistogram(requestDurationHistogram, metric.WithUnit("ms")).Record(ctx, duration.Milliseconds())

		responseID := request.IDResponseValidCount
		if err != nil {
			responseID = request.IDResponseErrorsCount
			if s, ok := status.FromError(err); ok {
				switch s.Code() {
				case codes.Unauthenticated:
					m.inc(ctx, legacyMetricsPrefix, request.IDResponseErrorsUnauthorized)
				case codes.DeadlineExceeded, codes.Canceled:
					m.inc(ctx, legacyMetricsPrefix, request.IDResponseErrorsTimeout)
				case codes.ResourceExhausted:
					m.inc(ctx, legacyMetricsPrefix, request.IDResponseErrorsRateLimit)
				}
			}
		}
		m.inc(ctx, legacyMetricsPrefix, responseID)
		return resp, err
	}
}

func (m *metricsInterceptor) inc(ctx context.Context, legacyMetricsPrefix string, id request.ResultID) {
	m.getCounter("grpc.server.", string(id)).Add(ctx, 1)
	m.getCounter(legacyMetricsPrefix, string(id)).Add(ctx, 1)
}

func (m *metricsInterceptor) getCounter(prefix, n string) metric.Int64Counter {
	name := prefix + n
	if met, ok := m.counters[name]; ok {
		return met
	}
	nm, _ := m.meter.Int64Counter(name)
	m.counters[name] = nm
	return nm
}

func (m *metricsInterceptor) getHistogram(n string, opts ...metric.Int64HistogramOption) metric.Int64Histogram {
	name := "grpc.server." + n
	if met, ok := m.histograms[name]; ok {
		return met
	}

	nm, _ := m.meter.Int64Histogram(name, opts...)
	m.histograms[name] = nm
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
	i := &metricsInterceptor{
		logger: logger,
		meter:  mp.Meter("github.com/elastic/apm-server/internal/beater/interceptors"),

		counters:   map[string]metric.Int64Counter{},
		histograms: map[string]metric.Int64Histogram{},
	}

	return i.Interceptor()
}
