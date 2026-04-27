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

func ExpectContainAndNotContainOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	notExpectedMetricKeys []string,
) {
	foundMetricKeys := assertOtelMetrics(t, reader, expectedMetrics, false, false)
	for _, key := range notExpectedMetricKeys {
		assert.NotContains(t, foundMetricKeys, key)
	}
}

func ExpectContainOtelMetricsKeys(t assert.TestingT, reader sdkmetric.Reader, expectedMetricsKeys []string) {
	expectedMetrics := make(map[string]any)
	for _, metricKey := range expectedMetricsKeys {
		expectedMetrics[metricKey] = nil
	}
	assertOtelMetrics(t, reader, expectedMetrics, false, true)
}

// assertOtelMetrics gathers all the metrics from `reader` and asserts that
// each gathered metric matches `expectedMetrics`.
//
// If `fullMatch` is true, every gathered counter / gauge / up-down counter
// must either be listed in `expectedMetrics` with the matching value, or
// implicitly equal 0; this is the natural pattern when using otelmetric
// helpers that zero-init counters at construction. Histograms are skipped
// from this rule because their "value" is an observation count and isn't
// meaningfully comparable to 0; tests that care about histogram counts
// must list them explicitly. `expectedMetrics` entries that don't appear
// in the gathered set are treated as failures.
//
// If `fullMatch` is false, `expectedMetrics` only needs to be a subset of
// the gathered metrics.
//
// If `skipValAssert` is true, the value assertion is skipped entirely;
// only the metric keys are checked.
func assertOtelMetrics(
	t assert.TestingT,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	fullMatch, skipValAssert bool,
) []string {
	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))
	var foundMetrics []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			value, isHistogram, ok := scalarValue(m.Data)
			if !ok {
				continue
			}
			foundMetrics = append(foundMetrics, m.Name)
			if skipValAssert {
				continue
			}
			if expected, listed := expectedMetrics[m.Name]; listed {
				assert.EqualValues(t, expected, value, m.Name)
			} else if fullMatch && !isHistogram {
				assert.EqualValues(t, 0, value, m.Name)
			}
		}
	}

	if fullMatch {
		// Catch the inverse: expected entries that didn't show up.
		for k := range expectedMetrics {
			assert.Contains(t, foundMetrics, k)
		}
	} else {
		var expectedKeys []string
		for k := range expectedMetrics {
			expectedKeys = append(expectedKeys, k)
		}
		assert.Subset(t, foundMetrics, expectedKeys)
	}
	return foundMetrics
}

// scalarValue extracts a single scalar value from a metricdata.Aggregation
// for assertion purposes. Histograms return their data point count and
// isHistogram=true; gauges and sums return the data point value with
// isHistogram=false. Anything with no data point or multiple data points
// is skipped (ok=false).
func scalarValue(data metricdata.Aggregation) (value any, isHistogram, ok bool) {
	switch d := data.(type) {
	case metricdata.Gauge[int64]:
		if len(d.DataPoints) != 1 {
			return nil, false, false
		}
		return d.DataPoints[0].Value, false, true
	case metricdata.Gauge[float64]:
		if len(d.DataPoints) != 1 {
			return nil, false, false
		}
		return d.DataPoints[0].Value, false, true
	case metricdata.Sum[int64]:
		if len(d.DataPoints) != 1 {
			return nil, false, false
		}
		return d.DataPoints[0].Value, false, true
	case metricdata.Histogram[int64]:
		if len(d.DataPoints) != 1 {
			return nil, false, false
		}
		return d.DataPoints[0].Count, true, true
	}
	return nil, false, false
}
