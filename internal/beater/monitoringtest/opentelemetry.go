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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// ExpectOtelMetrics asserts that the gathered metric set is exactly
// expectedMetrics: every gathered metric must be listed and match, and
// every listed metric must be present in the gathered set.
//
// Tests that exercise eagerly-zero-initialized counters can pass
// WithEagerPrefixes to declare the prefixes under which unlisted
// counters are tolerated provided their value is 0; histograms and
// non-zero counters under those prefixes still fail.
func ExpectOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	opts ...ExpectOption,
) {
	assertOtelMetrics(t, reader, expectedMetrics, true, false, opts...)
}

func ExpectContainOtelMetrics(
	t *testing.T,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
) {
	assertOtelMetrics(t, reader, expectedMetrics, false, false)
}

// ExpectOption configures ExpectOtelMetrics.
type ExpectOption func(*expectConfig)

type expectConfig struct {
	eagerPrefixes []string
}

// WithEagerPrefixes declares that the implementation eagerly
// zero-initializes counters under the given metric-name prefixes. An
// unlisted gathered counter whose name has one of these prefixes is
// required to equal 0 (and is otherwise tolerated); histograms and
// counters outside these prefixes still trigger "unexpected metric"
// failures.
func WithEagerPrefixes(prefixes ...string) ExpectOption {
	return func(c *expectConfig) {
		c.eagerPrefixes = append(c.eagerPrefixes, prefixes...)
	}
}

func hasEagerPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
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

// assertOtelMetrics gathers all the metrics from `reader` and asserts that the value of those gathered metrics
// are equal to that specified in `expectedMetrics`.
//
// If `fullMatch` is true, every gathered metric must be listed in
// `expectedMetrics` with a matching value, except counters whose name has
// a prefix passed via WithEagerPrefixes — those are tolerated when
// unlisted provided the value is 0. Otherwise, `expectedMetrics` only
// needs to be a subset of the gathered metrics.
//
// If `skipValAssert` is true, the value assertion will be skipped entirely i.e. only care about the metric keys.
func assertOtelMetrics(
	t assert.TestingT,
	reader sdkmetric.Reader,
	expectedMetrics map[string]any,
	fullMatch, skipValAssert bool,
	opts ...ExpectOption,
) []string {
	var cfg expectConfig
	for _, opt := range opts {
		opt(&cfg)
	}
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
			if fullMatch {
				if !isHistogram && hasEagerPrefix(m.Name, cfg.eagerPrefixes) {
					assert.EqualValues(t, 0, value, m.Name)
					continue
				}
				assert.Fail(t, "unexpected metric", m.Name)
			}
		}
	}

	var expectedMetricsKeys []string
	for k := range expectedMetrics {
		expectedMetricsKeys = append(expectedMetricsKeys, k)
	}
	// Subset (every expected is in found) covers both modes: the
	// per-metric "unexpected" check above already catches extras under
	// fullMatch, modulo the eager-prefix tolerance.
	assert.Subset(t, foundMetrics, expectedMetricsKeys)
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
