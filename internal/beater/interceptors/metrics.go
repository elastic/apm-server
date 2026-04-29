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
	"github.com/elastic/apm-server/internal/otelmetric"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	requestDurationHistogram = "request.duration"
	grpcServerPrefix         = "grpc.server."
)

// otlpGRPCLegacyMetricsPrefixes maps each OTLP gRPC service method to its
// legacy "apm-server.otlp.grpc.<signal>." metric prefix. Single source of
// truth for both the per-call dispatch in Interceptor() and the eager
// registration loop in Metrics(); adding a new signal means one entry.
var otlpGRPCLegacyMetricsPrefixes = map[string]string{
	"/opentelemetry.proto.collector.metrics.v1.MetricsService/Export": "apm-server.otlp.grpc.metrics.",
	"/opentelemetry.proto.collector.trace.v1.TraceService/Export":     "apm-server.otlp.grpc.traces.",
	"/opentelemetry.proto.collector.logs.v1.LogsService/Export":       "apm-server.otlp.grpc.logs.",
}

// counterKey identifies a counter by its (prefix, ResultID) tuple. Used as
// the map key in metricsInterceptor.counters; struct keys avoid the
// per-call allocation of "prefix + id" string concatenation.
type counterKey struct {
	prefix string
	id     request.ResultID
}

type metricsInterceptor struct {
	logger *logp.Logger

	// counters holds every counter the interceptor will ever record
	// against. Populated once in Metrics() before the interceptor is
	// returned; read-only thereafter, which makes plain map access safe
	// for concurrent reads. Lookup misses are reported via logger and
	// otherwise treated as no-ops; see the drift contract.
	counters            map[counterKey]metric.Int64Counter
	requestDurationHist metric.Int64Histogram
}

// Drift contract: every prefix the inner closure passes to m.inc() comes
// from otlpGRPCLegacyMetricsPrefixes (via the lookup below); every
// request.ResultID it passes must appear in request.AllResultIDs. The
// eager-registration loop in Metrics() iterates the cross-product of
// those two sets and pre-populates m.counters. Any (prefix, id) pair the
// inner closure tries to use that wasn't pre-populated is reported via
// logger.Error and otherwise dropped: drift surfaces at runtime instead
// of silently creating a counter on first use.
func (m *metricsInterceptor) Interceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		legacyMetricsPrefix, ok := otlpGRPCLegacyMetricsPrefixes[info.FullMethod]
		if !ok {
			m.logger.With(
				"grpc.request.method", info.FullMethod,
			).Warn("metrics registry missing")
			return handler(ctx, req)
		}

		m.inc(legacyMetricsPrefix, request.IDRequestCount)
		defer m.inc(legacyMetricsPrefix, request.IDResponseCount)

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		m.requestDurationHist.Record(context.Background(), duration.Milliseconds())

		responseID := request.IDResponseValidCount
		if err != nil {
			responseID = request.IDResponseErrorsCount
			if s, ok := status.FromError(err); ok {
				switch s.Code() {
				case codes.Unauthenticated:
					m.inc(legacyMetricsPrefix, request.IDResponseErrorsUnauthorized)
				case codes.DeadlineExceeded, codes.Canceled:
					m.inc(legacyMetricsPrefix, request.IDResponseErrorsTimeout)
				case codes.ResourceExhausted:
					m.inc(legacyMetricsPrefix, request.IDResponseErrorsRateLimit)
				}
			}
		}
		m.inc(legacyMetricsPrefix, responseID)
		return resp, err
	}
}

// inc increments the grpc.server.<id> and <legacyMetricsPrefix><id>
// counters. Both must already exist in m.counters, populated by Metrics()
// at construction; a missing entry is a drift bug and is logged.
func (m *metricsInterceptor) inc(legacyMetricsPrefix string, id request.ResultID) {
	server, sok := m.counters[counterKey{prefix: grpcServerPrefix, id: id}]
	legacy, lok := m.counters[counterKey{prefix: legacyMetricsPrefix, id: id}]
	if !sok || !lok {
		m.logger.With(
			"prefix", legacyMetricsPrefix,
			"id", string(id),
		).Error("monitoring counter not eagerly registered")
		return
	}
	ctx := context.Background()
	server.Add(ctx, 1)
	legacy.Add(ctx, 1)
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
	meter := mp.Meter("github.com/elastic/apm-server/internal/beater/interceptors")
	requestDurationHist, _ := meter.Int64Histogram(
		grpcServerPrefix+requestDurationHistogram,
		metric.WithUnit("ms"),
	)

	// Eager registration: pre-populate every (prefix, id) pair the
	// interceptor can ever record against. Done once before the
	// interceptor is returned, so subsequent map reads are concurrent-safe.
	counters := make(map[counterKey]metric.Int64Counter, len(request.AllResultIDs)*(len(otlpGRPCLegacyMetricsPrefixes)+1))
	for _, id := range request.AllResultIDs {
		counters[counterKey{prefix: grpcServerPrefix, id: id}] = otelmetric.NewInt64Counter(meter, grpcServerPrefix+string(id))
		for _, prefix := range otlpGRPCLegacyMetricsPrefixes {
			counters[counterKey{prefix: prefix, id: id}] = otelmetric.NewInt64Counter(meter, prefix+string(id))
		}
	}

	i := &metricsInterceptor{
		logger:              logger,
		counters:            counters,
		requestDurationHist: requestDurationHist,
	}
	return i.Interceptor()
}
