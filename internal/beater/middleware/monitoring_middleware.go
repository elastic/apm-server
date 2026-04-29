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

package middleware

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/otelmetric"
)

const (
	requestDurationHistogram = "request.duration"
	httpServerPrefix         = "http.server."
)

type monitoringMiddleware struct {
	logger              *logp.Logger
	legacyMetricsPrefix string
	serverCounters      map[request.ResultID]metric.Int64Counter
	legacyCounters      map[request.ResultID]metric.Int64Counter
	requestDurationHist metric.Int64Histogram
}

// Middleware returns the request.Middleware that increments per-request
// counters and records the request-duration histogram. Any counter or
// histogram it touches must already exist in the maps populated by
// MonitoringMiddleware(); a missing entry is an apm-server bug logged
// by inc().
func (m *monitoringMiddleware) Middleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			m.inc(request.IDRequestCount)

			start := time.Now()
			h(c)
			duration := time.Since(start)
			m.requestDurationHist.Record(context.Background(), duration.Milliseconds())

			m.inc(request.IDResponseCount)
			if c.Result.StatusCode >= http.StatusBadRequest {
				m.inc(request.IDResponseErrorsCount)
			} else {
				m.inc(request.IDResponseValidCount)
			}
			m.inc(c.Result.ID)
		}, nil

	}
}

// inc increments the http.server.<id> and <legacyMetricsPrefix><id>
// counters. A missing entry is an apm-server bug: MonitoringMiddleware()
// must eagerly create every counter inc() can use.
func (m *monitoringMiddleware) inc(id request.ResultID) {
	server, sok := m.serverCounters[id]
	legacy, lok := m.legacyCounters[id]
	if !sok || !lok {
		m.logger.With(
			"prefix", m.legacyMetricsPrefix,
			"id", string(id),
		).Error("BUG: monitoring counter missing from eager registration")
		return
	}
	ctx := context.Background()
	server.Add(ctx, 1)
	legacy.Add(ctx, 1)
}

// MonitoringMiddleware returns a middleware that increments monitoring
// counters for collecting metrics about request processing.
//
// All counters for the canonical request.AllResultIDs set are created
// eagerly; the maps are read-only after construction so plain map access
// is safe across concurrent requests. Any new ResultID introduced into
// Middleware()'s inc() paths must be added to AllResultIDs (or
// MapResultIDToStatus, which feeds it).
func MonitoringMiddleware(legacyMetricsPrefix string, mp metric.MeterProvider, logger *logp.Logger) Middleware {
	meter := mp.Meter("github.com/elastic/apm-server/internal/beater/middleware")
	requestDurationHist, _ := meter.Int64Histogram(
		httpServerPrefix+requestDurationHistogram,
		metric.WithUnit("ms"),
	)

	serverCounters := make(map[request.ResultID]metric.Int64Counter, len(request.AllResultIDs))
	legacyCounters := make(map[request.ResultID]metric.Int64Counter, len(request.AllResultIDs))
	for _, id := range request.AllResultIDs {
		serverCounters[id] = otelmetric.NewInt64Counter(meter, httpServerPrefix+string(id))
		legacyCounters[id] = otelmetric.NewInt64Counter(meter, legacyMetricsPrefix+string(id))
	}

	mid := &monitoringMiddleware{
		logger:              logger,
		legacyMetricsPrefix: legacyMetricsPrefix,
		serverCounters:      serverCounters,
		legacyCounters:      legacyCounters,
		requestDurationHist: requestDurationHist,
	}
	return mid.Middleware()
}
