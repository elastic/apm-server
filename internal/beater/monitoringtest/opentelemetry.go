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

	"github.com/elastic/apm-server/internal/beater/request"
)

// ExpectOtelMetrics asserts that the gathered metric set is exactly
// expectedMetrics: every gathered metric must be listed and match, and
// every listed metric must be present in the gathered set. Tests that
// exercise eagerly-zero-initialized counters (HTTP middleware, gRPC
// interceptor) bootstrap their expectedMetrics map via EagerCountersZeros
// so the noise floor is part of the asserted contract.
func ExpectOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, matchModeFull, false)
}

// ExpectOtelMetricsNonZero is a relaxation of ExpectOtelMetrics for tests
// whose code path exercises a large set of eagerly-zero-initialized
// counters that the test does not care to enumerate. Every listed metric
// is asserted exactly; every unlisted gathered metric must equal 0
// (histograms are exempt because their value is an observation count).
// Use ExpectOtelMetrics when the test wants a strict "exactly these
// metrics fired" contract; use this when the test only wants to assert
// "these specific metrics fired with these values, and nothing else
// surprising fired".
func ExpectOtelMetricsNonZero(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, matchModeNonZero, false)
}

func ExpectContainOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, matchModeContain, false)
}

func ExpectContainAndNotContainOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	notExpectedMetricKeys []string,
) {
	foundMetricKeys := assertOtelMetrics(t, reader, expectedMetrics, matchModeContain, false)
	for _, key := range notExpectedMetricKeys {
		assert.NotContains(t, foundMetricKeys, key)
	}
}

func ExpectContainOtelMetricsKeys(t assert.TestingT, reader sdkmetric.Reader, expectedMetricsKeys []string) {
	expectedMetrics := make(map[string]any)
	for _, metricKey := range expectedMetricsKeys {
		expectedMetrics[metricKey] = nil
	}
	assertOtelMetrics(t, reader, expectedMetrics, matchModeContain, true)
}

// EagerCountersZeros returns a map[name]int64(0) for every (prefix, id)
// pair in `prefixes × ids`. Tests that exercise code paths with eagerly
// zero-initialized counters use this to bootstrap their expected map for
// ExpectOtelMetrics; the listed prefixes make the eager-init contract
// part of the test, and any non-zero values are added by the caller via
// map assignment after.
func EagerCountersZeros(prefixes []string, ids []request.ResultID) map[string]any {
	out := make(map[string]any, len(prefixes)*len(ids))
	for _, p := range prefixes {
		for _, id := range ids {
			out[p+string(id)] = int64(0)
		}
	}
	return out
}

type matchMode int

const (
	// matchModeFull: gathered ≡ expected (set equality with values).
	matchModeFull matchMode = iota
	// matchModeNonZero: every listed metric matches, every unlisted
	// gathered metric must equal 0 (histograms exempt).
	matchModeNonZero
	// matchModeContain: expected is a subset of gathered.
	matchModeContain
)

// assertOtelMetrics gathers metrics from `reader` and asserts they match
// `expectedMetrics` according to `mode`.
//
// If `skipValAssert` is true, only the metric keys are checked.
func assertOtelMetrics(
	t assert.TestingT,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	mode matchMode,
	skipValAssert bool,
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
				continue
			}
			switch mode {
			case matchModeFull:
				assert.Fail(t, "unexpected metric", m.Name)
			case matchModeNonZero:
				if !isHistogram {
					assert.EqualValues(t, 0, value, m.Name)
				}
			}
		}
	}

	expectedKeys := make([]string, 0, len(expectedMetrics))
	for k := range expectedMetrics {
		expectedKeys = append(expectedKeys, k)
	}
	switch mode {
	case matchModeFull:
		assert.ElementsMatch(t, foundMetrics, expectedKeys)
	case matchModeNonZero, matchModeContain:
		assert.Subset(t, foundMetrics, expectedKeys)
	}
	return foundMetrics
}

// scalarValue extracts the assertable scalar from a metric aggregation:
// the data-point value for gauges and sums, the data-point count for
// histograms. Returns ok=false if the metric has 0 or >1 data points.
// isHistogram is true when the value is a histogram observation count;
// callers use it to skip "must be 0 if unlisted" rules that don't apply
// to histograms.
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
