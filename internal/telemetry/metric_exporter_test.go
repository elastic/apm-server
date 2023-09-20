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

package telemetry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/elastic/apm-data/model/modelpb"
)

func TestMetricExporterWithNoBatchProcessor(t *testing.T) {
	e := NewMetricExporter()
	var rm metricdata.ResourceMetrics
	err := e.Export(context.Background(), &rm)
	assert.NoError(t, err)
}

func TestMetricExporter(t *testing.T) {
	service := modelpb.Service{Name: "apm-server", Language: &modelpb.Language{Name: "go"}}

	for _, tt := range []struct {
		name string

		exporterConfig []ConfigOption
		recordMetrics  func(ctx context.Context, meter metric.Meter)
		expectedBatch  modelpb.Batch
	}{
		{
			name: "with a filtered metric",
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Int64Counter("filtered_metric")
				assert.NoError(t, err)
				counter.Add(ctx, 1)
			},
			expectedBatch: modelpb.Batch(nil),
		},
		{
			name: "with an int64 histogram",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"histogram_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Int64Histogram("histogram_metric")
				assert.NoError(t, err)
				counter.Record(ctx, 3, metric.WithAttributes(
					attribute.Key("code").String("200"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 10, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"code": {Value: "200"}, "method": {Value: "GET"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{
								Name: "histogram_metric",
								Type: modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
								Histogram: &modelpb.Histogram{
									Counts: []uint64{1},
									Values: []float64{3.414215},
								},
							},
						},
					},
				},
				{
					Service: &service,
					Labels:  modelpb.Labels{"code": {Value: "302"}, "method": {Value: "GET"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{
								Name: "histogram_metric",
								Type: modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
								Histogram: &modelpb.Histogram{
									Counts: []uint64{1},
									Values: []float64{9.65685},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "with a float64 histogram",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"histogram_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Float64Histogram("histogram_metric")
				assert.NoError(t, err)
				counter.Record(ctx, 3.4, metric.WithAttributes(
					attribute.Key("code").String("200"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 3.4, metric.WithAttributes(
					attribute.Key("code").String("200"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 3.4, metric.WithAttributes(
					attribute.Key("code").String("200"),
					attribute.Key("method").String("GET"),
				))

				counter.Record(ctx, 5.5, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 5.5, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 5.5, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))

				counter.Record(ctx, 11.2, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 11.2, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
				counter.Record(ctx, 11.2, metric.WithAttributes(
					attribute.Key("code").String("302"),
					attribute.Key("method").String("GET"),
				))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"code": {Value: "200"}, "method": {Value: "GET"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{
								Name: "histogram_metric",
								Type: modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
								Histogram: &modelpb.Histogram{
									Values: []float64{3.414215},
									Counts: []uint64{3},
								},
							},
						},
					},
				},
				{
					Service: &service,
					Labels:  modelpb.Labels{"code": {Value: "302"}, "method": {Value: "GET"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{
								Name: "histogram_metric",
								Type: modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
								Histogram: &modelpb.Histogram{
									Values: []float64{4.828425, 9.65685},
									Counts: []uint64{3, 3},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "with an int64 counter",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"sum_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Int64Counter("sum_metric")
				assert.NoError(t, err)
				counter.Add(ctx, 5, metric.WithAttributes(attribute.Key("A").String("B")))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"A": {Value: "B"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{Name: "sum_metric", Value: 5, Type: modelpb.MetricType_METRIC_TYPE_COUNTER},
						},
					},
				},
			},
		},
		{
			name: "with a float64 counter",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"sum_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Float64Counter("sum_metric")
				assert.NoError(t, err)
				counter.Add(ctx, 3.14, metric.WithAttributes(attribute.Key("A").String("B")))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"A": {Value: "B"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{Name: "sum_metric", Value: 3.14, Type: modelpb.MetricType_METRIC_TYPE_COUNTER},
						},
					},
				},
			},
		},
		{
			name: "with an int64 gauge",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"gauge_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Int64UpDownCounter("gauge_metric")
				assert.NoError(t, err)
				counter.Add(ctx, 5, metric.WithAttributes(attribute.Key("A").String("B")))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"A": {Value: "B"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{Name: "gauge_metric", Value: 5, Type: modelpb.MetricType_METRIC_TYPE_COUNTER},
						},
					},
				},
			},
		},
		{
			name: "with a float64 gauge",
			exporterConfig: []ConfigOption{
				WithMetricFilter([]string{"gauge_metric"}),
			},
			recordMetrics: func(ctx context.Context, meter metric.Meter) {
				counter, err := meter.Float64UpDownCounter("gauge_metric")
				assert.NoError(t, err)
				counter.Add(ctx, 3.14, metric.WithAttributes(attribute.Key("A").String("B")))
			},
			expectedBatch: modelpb.Batch{
				{
					Service: &service,
					Labels:  modelpb.Labels{"A": {Value: "B"}},
					Metricset: &modelpb.Metricset{
						Name: "app",
						Samples: []*modelpb.MetricsetSample{
							{Name: "gauge_metric", Value: 3.14, Type: modelpb.MetricType_METRIC_TYPE_COUNTER},
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var batch modelpb.Batch

			p := modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error {
				batch = append(batch, (*b)...)
				return nil
			})
			tt.exporterConfig = append(tt.exporterConfig, WithBatchProcessor(p))
			e := NewMetricExporter(tt.exporterConfig...)

			provider := sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(sdkmetric.NewPeriodicReader(e)),
			)
			meter := provider.Meter("test")

			tt.recordMetrics(ctx, meter)

			provider.Shutdown(context.Background())

			assertEventsMatch(t, tt.expectedBatch, batch)
		})
	}
}

func TestMetricExporterSetBatchProcessor(t *testing.T) {
	p := modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error {
		return nil
	})
	e := NewMetricExporter()
	assert.Nil(t, e.processor)
	e.SetBatchProcessor(p)
	assert.NotNil(t, e.processor)
}

func assertEventsMatch(t *testing.T, expected []*modelpb.APMEvent, actual []*modelpb.APMEvent) {
	t.Helper()
	sort.Slice(expected, func(i, j int) bool {
		return strings.Compare(expected[i].String(), expected[j].String()) == -1
	})
	sort.Slice(actual, func(i, j int) bool {
		return strings.Compare(actual[i].String(), actual[j].String()) == -1
	})

	now := modelpb.FromTime(time.Now())
	for i, e := range actual {
		assert.InDelta(t, now, e.Timestamp, float64((2 * time.Second).Nanoseconds()))
		e.Timestamp = 0
		assert.InDelta(t, now, e.Event.Received, float64((2 * time.Second).Nanoseconds()))
		e.Event.Received = 0
		if len(expected) > i && expected[i].Event == nil {
			e.Event = nil
		}
	}

	diff := cmp.Diff(
		expected, actual,
		protocmp.Transform(),
		// Ignore order of events and their metrics. Some other slices
		// have a defined order (e.g. histogram counts/values), so we
		// don't ignore the order of all slices.
		//
		// Comparing string representations is a bit of a hack; ideally
		// we would use like https://github.com/google/go-cmp/issues/67
		protocmp.SortRepeated(func(x, y *modelpb.MetricsetSample) bool {
			return fmt.Sprint(x) < fmt.Sprint(y)
		}),
	)
	if diff != "" {
		t.Fatal(diff)
	}
}
