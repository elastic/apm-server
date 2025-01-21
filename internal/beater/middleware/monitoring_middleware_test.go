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
	"sync"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/beater/request"
)

var (
	mockMonitoringRegistry = monitoring.Default.NewRegistry("mock.monitoring")
	mockMonitoringNil      = map[request.ResultID]*monitoring.Int{}
)

func TestMonitoringHandler(t *testing.T) {
	checkMonitoring := func(t *testing.T,
		h func(*request.Context),
		expectedOtel map[string]interface{},
	) {
		reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
			func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
				return metricdata.DeltaTemporality
			},
		))
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

		c, _ := DefaultContextWithResponseRecorder()
		Apply(MonitoringMiddleware("", mp), h)(c)

		monitoringtest.ExpectOtelMetrics(t, reader, expectedOtel)
	}

	t.Run("Error", func(t *testing.T) {
		checkMonitoring(t,
			Handler403,
			map[string]interface{}{
				"http.server." + string(request.IDRequestCount):            1,
				"http.server." + string(request.IDResponseCount):           1,
				"http.server." + string(request.IDResponseErrorsCount):     1,
				"http.server." + string(request.IDResponseErrorsForbidden): 1,
				string(request.IDRequestCount):                             1,
				string(request.IDResponseCount):                            1,
				string(request.IDResponseErrorsCount):                      1,
				string(request.IDResponseErrorsForbidden):                  1,

				"http.server.request.duration": 1,
			},
		)
	})

	t.Run("Accepted", func(t *testing.T) {
		checkMonitoring(t,
			Handler202,
			map[string]interface{}{
				"http.server." + string(request.IDRequestCount):          1,
				"http.server." + string(request.IDResponseCount):         1,
				"http.server." + string(request.IDResponseValidCount):    1,
				"http.server." + string(request.IDResponseValidAccepted): 1,
				string(request.IDRequestCount):                           1,
				string(request.IDResponseCount):                          1,
				string(request.IDResponseValidCount):                     1,
				string(request.IDResponseValidAccepted):                  1,

				"http.server.request.duration": 1,
			},
		)
	})

	t.Run("Idle", func(t *testing.T) {
		checkMonitoring(t,
			HandlerIdle,
			map[string]interface{}{
				"http.server." + string(request.IDRequestCount):       1,
				"http.server." + string(request.IDResponseCount):      1,
				"http.server." + string(request.IDResponseValidCount): 1,
				"http.server." + string(request.IDUnset):              1,
				string(request.IDRequestCount):                        1,
				string(request.IDResponseCount):                       1,
				string(request.IDResponseValidCount):                  1,
				string(request.IDUnset):                               1,

				"http.server.request.duration": 1,
			},
		)
	})

	t.Run("Panic", func(t *testing.T) {
		checkMonitoring(t,
			Apply(RecoverPanicMiddleware(), HandlerPanic),
			map[string]interface{}{
				"http.server." + string(request.IDRequestCount):           1,
				"http.server." + string(request.IDResponseCount):          1,
				"http.server." + string(request.IDResponseErrorsCount):    1,
				"http.server." + string(request.IDResponseErrorsInternal): 1,
				string(request.IDRequestCount):                            1,
				string(request.IDResponseCount):                           1,
				string(request.IDResponseErrorsCount):                     1,
				string(request.IDResponseErrorsInternal):                  1,

				"http.server.request.duration": 1,
			},
		)
	})

	t.Run("Nil", func(t *testing.T) {
		checkMonitoring(t,
			HandlerIdle,
			map[string]interface{}{
				"http.server." + string(request.IDRequestCount):       1,
				"http.server." + string(request.IDResponseCount):      1,
				"http.server." + string(request.IDResponseValidCount): 1,
				"http.server." + string(request.IDUnset):              1,
				string(request.IDRequestCount):                        1,
				string(request.IDResponseCount):                       1,
				string(request.IDResponseValidCount):                  1,
				string(request.IDUnset):                               1,

				"http.server.request.duration": 1,
			},
		)
	})

	t.Run("Parallel", func(t *testing.T) {
		const i = 3
		reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
			func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
				return metricdata.DeltaTemporality
			},
		))
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		m := MonitoringMiddleware("", mp)
		c, _ := DefaultContextWithResponseRecorder()
		var wg sync.WaitGroup
		for range i {
			wg.Add(1)
			go func() {
				Apply(m, HandlerIdle)(c)
				wg.Done()
			}()
		}
		wg.Wait()
		monitoringtest.ExpectOtelMetrics(t, reader, map[string]interface{}{
			"http.server." + string(request.IDRequestCount):       i,
			"http.server." + string(request.IDResponseCount):      i,
			"http.server." + string(request.IDResponseValidCount): i,
			"http.server." + string(request.IDUnset):              i,
			string(request.IDRequestCount):                        i,
			string(request.IDResponseCount):                       i,
			string(request.IDResponseValidCount):                  i,
			string(request.IDUnset):                               i,

			"http.server.request.duration": i,
		})
	})
}
