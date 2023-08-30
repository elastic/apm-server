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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-server/internal/beater/request"
)

func TestMonitoringHandler(t *testing.T) {
	checkMonitoring := func(t *testing.T,
		h func(*request.Context),
		expected map[request.ResultID]int64,
	) {
		reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
			func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
				return metricdata.DeltaTemporality
			},
		))
		otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

		c, _ := DefaultContextWithResponseRecorder()
		Apply(MonitoringMiddleware("prefix"), h)(c)

		var rm metricdata.ResourceMetrics
		assert.NoError(t, reader.Collect(context.Background(), &rm))

		for _, sm := range rm.ScopeMetrics {
			assert.Equal(t, len(expected), len(sm.Metrics))

			for _, m := range sm.Metrics {
				switch d := m.Data.(type) {
				case metricdata.Sum[int64]:
					assert.Equal(t, 1, len(d.DataPoints))
					_, name, _ := strings.Cut(m.Name, "prefix.")

					if v, ok := expected[request.ResultID(name)]; ok {
						assert.Equal(t, v, d.DataPoints[0].Value)
					} else {
						assert.Fail(t, "unexpected metric", m.Name)
					}
				}
			}
		}
	}

	t.Run("Error", func(t *testing.T) {
		checkMonitoring(t,
			Handler403,
			map[request.ResultID]int64{
				request.IDRequestCount:            1,
				request.IDResponseCount:           1,
				request.IDResponseErrorsCount:     1,
				request.IDResponseErrorsForbidden: 1,
			},
		)
	})

	t.Run("Accepted", func(t *testing.T) {
		checkMonitoring(t,
			Handler202,
			map[request.ResultID]int64{
				request.IDRequestCount:          1,
				request.IDResponseCount:         1,
				request.IDResponseValidCount:    1,
				request.IDResponseValidAccepted: 1,
			},
		)
	})

	t.Run("Idle", func(t *testing.T) {
		checkMonitoring(t,
			HandlerIdle,
			map[request.ResultID]int64{
				request.IDRequestCount:       1,
				request.IDResponseCount:      1,
				request.IDResponseValidCount: 1,
				request.IDUnset:              1,
			},
		)
	})

	t.Run("Panic", func(t *testing.T) {
		checkMonitoring(t,
			Apply(RecoverPanicMiddleware(), HandlerPanic),
			map[request.ResultID]int64{
				request.IDRequestCount:           1,
				request.IDResponseCount:          1,
				request.IDResponseErrorsCount:    1,
				request.IDResponseErrorsInternal: 1,
			},
		)
	})
}
