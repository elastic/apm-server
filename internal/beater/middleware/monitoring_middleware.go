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
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-server/internal/beater/request"
)

const (
	requestDurationHistogram = "request.duration"
)

type monitoringMiddleware struct {
	meter               metric.Meter
	legacyMetricsPrefix string

	counters   sync.Map
	histograms sync.Map
}

func (m *monitoringMiddleware) Middleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			m.inc(request.IDRequestCount)

			start := time.Now()
			h(c)
			duration := time.Since(start)
			m.getHistogram(requestDurationHistogram, metric.WithUnit("ms")).Record(context.Background(), duration.Milliseconds())

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

func (m *monitoringMiddleware) inc(id request.ResultID) {
	m.getCounter("http.server.", string(id)).Add(context.Background(), 1)
	m.getCounter(m.legacyMetricsPrefix, string(id)).Add(context.Background(), 1)
}

func (m *monitoringMiddleware) getCounter(prefix, name string) metric.Int64Counter {
	name = prefix + name
	if met, ok := m.counters.Load(name); ok {
		return met.(metric.Int64Counter)
	}
	nm, _ := m.meter.Int64Counter(name)
	met, _ := m.counters.LoadOrStore(name, nm)
	return met.(metric.Int64Counter)
}

func (m *monitoringMiddleware) getHistogram(n string, opts ...metric.Int64HistogramOption) metric.Int64Histogram {
	name := "http.server." + n
	if met, ok := m.histograms.Load(name); ok {
		return met.(metric.Int64Histogram)
	}

	nm, _ := m.meter.Int64Histogram(name, opts...)
	met, _ := m.histograms.LoadOrStore(name, nm)
	return met.(metric.Int64Histogram)
}

// MonitoringMiddleware returns a middleware that increases monitoring counters for collecting metrics
// about request processing. As input parameter it takes a map capable of mapping a request.ResultID to a counter.
func MonitoringMiddleware(legacyMetricsPrefix string, mp metric.MeterProvider) Middleware {
	mid := &monitoringMiddleware{
		meter:               mp.Meter("github.com/elastic/apm-server/internal/beater/middleware"),
		legacyMetricsPrefix: legacyMetricsPrefix,
		counters:            sync.Map{},
		histograms:          sync.Map{},
	}

	return mid.Middleware()
}
