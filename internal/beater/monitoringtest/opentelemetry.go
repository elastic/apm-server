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

package monitoringtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func ExpectOtelMetrics(t *testing.T, reader sdkmetric.Reader, expectedMetrics map[string]interface{}) {
	t.Helper()

	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))
	foundMetrics := []string{}
	for _, sm := range rm.ScopeMetrics {

		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)

				if v, ok := expectedMetrics[m.Name]; ok {
					if dp, ok := v.(int); ok {
						assert.Equal(t, int64(dp), d.DataPoints[0].Value, m.Name)
					} else {
						assert.Fail(t, "expected an int value", m.Name)
					}
				} else {
					assert.Fail(t, "unexpected metric", m.Name)
				}
			case metricdata.Histogram[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)

				if v, ok := expectedMetrics[m.Name]; ok {
					if dp, ok := v.(int); ok {
						assert.Equal(t, uint64(dp), d.DataPoints[0].Count, m.Name)
					} else {
						assert.Fail(t, "expected an int value", m.Name)
					}
				} else {
					assert.Fail(t, "unexpected metric", m.Name)
				}
			}
		}
	}

	expectedMetricsKeys := []string{}
	for k := range expectedMetrics {
		expectedMetricsKeys = append(expectedMetricsKeys, k)
	}
	assert.ElementsMatch(t, expectedMetricsKeys, foundMetrics)
}
