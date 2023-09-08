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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/elastic-agent-libs/monitoring"
)

const (
	requestDurationHistogram = "request.duration"
)

type monitoringMiddleware struct {
	meter metric.Meter

	ints       map[request.ResultID]*monitoring.Int
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Int64Histogram
}

func (m *monitoringMiddleware) Middleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			ctx := context.Background()

			m.getCounter(string(request.IDRequestCount)).Add(ctx, 1)
			m.inc(request.IDRequestCount)

			start := time.Now()
			h(c)
			duration := time.Since(start)
			m.getHistogram(requestDurationHistogram, metric.WithUnit("ms")).Record(ctx, duration.Milliseconds())

			m.getCounter(string(request.IDResponseCount)).Add(ctx, 1)
			m.inc(request.IDResponseCount)
			if c.Result.StatusCode >= http.StatusBadRequest {
				m.getCounter(string(request.IDResponseErrorsCount)).Add(ctx, 1)
				m.inc(request.IDResponseErrorsCount)
			} else {
				m.getCounter(string(request.IDResponseValidCount)).Add(ctx, 1)
				m.inc(request.IDResponseValidCount)
			}

			m.getCounter(string(c.Result.ID)).Add(ctx, 1)
			m.inc(c.Result.ID)
		}, nil

	}
}

func (m *monitoringMiddleware) inc(id request.ResultID) {
	if counter, ok := m.ints[id]; ok {
		counter.Inc()
	}
}

func (m *monitoringMiddleware) getCounter(n string) metric.Int64Counter {
	name := "http.server." + n
	if met, ok := m.counters[name]; ok {
		return met
	}

	nm, _ := m.meter.Int64Counter(name)
	m.counters[name] = nm
	return nm
}

func (m *monitoringMiddleware) getHistogram(n string, opts ...metric.Int64HistogramOption) metric.Int64Histogram {
	name := "http.server." + n
	if met, ok := m.histograms[name]; ok {
		return met
	}

	nm, _ := m.meter.Int64Histogram(name, opts...)
	m.histograms[name] = nm
	return nm
}

// MonitoringMiddleware returns a middleware that increases monitoring counters for collecting metrics
// about request processing. As input parameter it takes a map capable of mapping a request.ResultID to a counter.
func MonitoringMiddleware(m map[request.ResultID]*monitoring.Int, mp metric.MeterProvider) Middleware {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	mid := &monitoringMiddleware{
		meter:      mp.Meter("internal/beater/middleware"),
		ints:       m,
		counters:   map[string]metric.Int64Counter{},
		histograms: map[string]metric.Int64Histogram{},
	}

	return mid.Middleware()
}
