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

// Package otelmetric provides thin wrappers around OpenTelemetry metric
// instrument constructors that record a zero data point at creation time.
//
// OTel only emits an instrument's value once it has at least one observed
// data point. For long-tail counters (rare error paths, conditional
// features) that means /stats won't list them until the relevant code
// path has fired at least once. Adding zero to each counter at
// construction makes the instrument enumerable by /stats from process
// start, so downstream tooling that consumes /stats as a metric
// definition (metricbeat field mappings, monitoring templates,
// integration packages) sees a stable shape regardless of runtime
// traffic.
//
// Only counter-shaped instruments are wrapped here. Adding zero to a
// counter is a semantic no-op, so the eager registration is always
// correct. Gauges, by contrast, represent a *current* value: recording
// zero would falsely declare "the current value is 0" until something
// records a real one. Histograms record observations, so an eager zero
// would inflate observation counts. Callers needing those instrument
// kinds should construct them directly and decide on their own initial
// value (or rely on a periodic reporter to fill them in).
//
// The error returned by the underlying constructors is intentionally
// dropped: it only surfaces invalid instrument names, which is a static
// programmer error rather than a runtime condition.
package otelmetric

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

// NewInt64Counter creates an Int64Counter and adds 0 once so the
// instrument is enumerated by exporters from process start.
func NewInt64Counter(meter metric.Meter, name string, options ...metric.Int64CounterOption) metric.Int64Counter {
	c, _ := meter.Int64Counter(name, options...)
	c.Add(context.Background(), 0)
	return c
}

// NewInt64UpDownCounter creates an Int64UpDownCounter and adds 0 once so
// the instrument is enumerated by exporters from process start.
func NewInt64UpDownCounter(meter metric.Meter, name string, options ...metric.Int64UpDownCounterOption) metric.Int64UpDownCounter {
	c, _ := meter.Int64UpDownCounter(name, options...)
	c.Add(context.Background(), 0)
	return c
}
