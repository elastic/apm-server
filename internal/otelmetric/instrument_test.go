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

package otelmetric_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-server/internal/otelmetric"
)

// TestEagerEmit verifies that the wrapper constructors emit each instrument
// even when nothing has explicitly observed a value, which is the property
// downstream tooling depends on to enumerate /stats as a metric definition.
func TestEagerEmit(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := mp.Meter("test")

	otelmetric.NewInt64Counter(meter, "test.counter")
	otelmetric.NewInt64UpDownCounter(meter, "test.updown")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	got := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if d, ok := m.Data.(metricdata.Sum[int64]); ok {
				require.Len(t, d.DataPoints, 1, m.Name)
				got[m.Name] = d.DataPoints[0].Value
			}
		}
	}
	assert.Equal(t, map[string]int64{
		"test.counter": 0,
		"test.updown":  0,
	}, got)
}
