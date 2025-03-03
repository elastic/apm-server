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

<<<<<<< HEAD
func ExpectOtelMetrics(t *testing.T, reader sdkmetric.Reader, expectedMetrics map[string]interface{}) {
	assertOtelMetrics(t, reader, expectedMetrics, true)
}

func ExpectContainOtelMetrics(t *testing.T, reader sdkmetric.Reader, expectedMetrics map[string]interface{}) {
	assertOtelMetrics(t, reader, expectedMetrics, false)
}

func assertOtelMetrics(t *testing.T, reader sdkmetric.Reader, expectedMetrics map[string]interface{}, match bool) {
=======
func ExpectOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, true, false)
}

func ExpectContainOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, false, false)
}

func ExpectContainOtelMetricsKeys(t *testing.T, reader sdkmetric.Reader, expectedMetricsKeys []string) {
	expectedMetrics := make(map[string]any)
	for _, metricKey := range expectedMetricsKeys {
		expectedMetrics[metricKey] = nil
	}
	assertOtelMetrics(t, reader, expectedMetrics, false, true)
}

// assertOtelMetrics gathers all the metrics from `reader` and asserts that the value of those gathered metrics
// are equal to that specified in `expectedMetrics`.
//
// If `fullMatch` is true, all gathered metrics from `reader` must be found in `expectedMetrics` and vice versa.
// Otherwise, `expectedMetrics` only need to be a subset of the gathered metrics.
//
// If `skipValAssert` is true, the value assertion will be skipped entirely i.e. only care about the metric keys.
func assertOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	fullMatch, skipValAssert bool,
) {
>>>>>>> a5a53ecd (test: Fix flaky esoutput (#15948))
	t.Helper()

	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))
	foundMetrics := []string{}
	for _, sm := range rm.ScopeMetrics {

		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Gauge[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)
<<<<<<< HEAD
=======
				if skipValAssert {
					continue
				}
>>>>>>> a5a53ecd (test: Fix flaky esoutput (#15948))

				if v, ok := expectedMetrics[m.Name]; ok {
					assert.EqualValues(t, v, d.DataPoints[0].Value, m.Name)
				} else if fullMatch {
					assert.Fail(t, "unexpected metric", m.Name)
				}

			case metricdata.Sum[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)
<<<<<<< HEAD
=======
				if skipValAssert {
					continue
				}
>>>>>>> a5a53ecd (test: Fix flaky esoutput (#15948))

				if v, ok := expectedMetrics[m.Name]; ok {
					assert.EqualValues(t, v, d.DataPoints[0].Value, m.Name)
				} else if fullMatch {
					assert.Fail(t, "unexpected metric", m.Name)
				}

			case metricdata.Histogram[int64]:
				assert.Equal(t, 1, len(d.DataPoints))
				foundMetrics = append(foundMetrics, m.Name)
<<<<<<< HEAD
=======
				if skipValAssert {
					continue
				}
>>>>>>> a5a53ecd (test: Fix flaky esoutput (#15948))

				if v, ok := expectedMetrics[m.Name]; ok {
					assert.EqualValues(t, v, d.DataPoints[0].Count, m.Name)
				} else if fullMatch {
					assert.Fail(t, "unexpected metric", m.Name)
				}
			}
		}
	}

	expectedMetricsKeys := []string{}
	for k := range expectedMetrics {
		expectedMetricsKeys = append(expectedMetricsKeys, k)
	}
	if fullMatch {
		assert.ElementsMatch(t, expectedMetricsKeys, foundMetrics)
	} else {
		assert.Subset(t, foundMetrics, expectedMetricsKeys)
	}
}
