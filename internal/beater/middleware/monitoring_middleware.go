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
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-server/internal/beater/request"
)

var usedResults = []request.ResultID{
	request.IDRequestCount,
	request.IDResponseCount,
	request.IDResponseErrorsCount,
	request.IDResponseValidCount,
}

// MonitoringMiddleware returns a middleware that increases monitoring counters for collecting metrics
// about request processing. As input parameter it takes a string that will be prepended to every metric name.
func MonitoringMiddleware(metricPrefix string) Middleware {
	meter := otel.Meter("internal/beater/middleware")
	metrics := map[request.ResultID]metric.Int64Counter{}

	for _, v := range usedResults {
		nm, err := meter.Int64Counter(strings.Join([]string{metricPrefix, string(v)}, "."))
		if err == nil {
			metrics[v] = nm
		}
	}

	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			ctx := context.Background()

			metrics[request.IDRequestCount].Add(ctx, 1)

			h(c)

			metrics[request.IDResponseCount].Add(ctx, 1)
			if c.Result.StatusCode >= http.StatusBadRequest {
				metrics[request.IDResponseErrorsCount].Add(ctx, 1)
			} else {
				metrics[request.IDResponseValidCount].Add(ctx, 1)
			}

			nm, err := meter.Int64Counter(strings.Join([]string{metricPrefix, string(c.Result.ID)}, "."))
			if err == nil {
				nm.Add(ctx, 1)
			}
		}, nil

	}
}
