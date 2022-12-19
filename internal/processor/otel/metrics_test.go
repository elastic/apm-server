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

// Portions copied from OpenTelemetry Collector (contrib), from the
// elastic exporter.
//
// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel_test

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/processor/otel"
	"github.com/elastic/elastic-agent-libs/logp"
)

func TestConsumeMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metricSlice := scopeMetrics.Metrics()
	appendMetric := func(name string) pmetric.Metric {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		return metric
	}

	timestamp0 := time.Unix(123, 0).UTC()
	timestamp1 := time.Unix(456, 0).UTC()

	var expectDropped int64

	gauge := appendMetric("gauge_metric").SetEmptyGauge()
	gaugeDP0 := gauge.DataPoints().AppendEmpty()
	gaugeDP0.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	gaugeDP0.SetIntValue(1)
	gaugeDP1 := gauge.DataPoints().AppendEmpty()
	gaugeDP1.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	gaugeDP1.SetDoubleValue(2.3)
	gaugeDP1.Attributes().PutStr("k", "v")
	gaugeDP2 := gauge.DataPoints().AppendEmpty()
	gaugeDP2.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	gaugeDP2.SetIntValue(4)
	gaugeDP3 := gauge.DataPoints().AppendEmpty()
	gaugeDP3.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	gaugeDP3.SetDoubleValue(5.6)
	gaugeDP3.Attributes().PutStr("k", "v2")

	sum := appendMetric("sum_metric").SetEmptySum()
	sumDP0 := sum.DataPoints().AppendEmpty()
	sumDP0.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	sumDP0.SetIntValue(7)
	sumDP1 := sum.DataPoints().AppendEmpty()
	sumDP1.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	sumDP1.SetDoubleValue(8.9)
	sumDP1.Attributes().PutStr("k", "v")
	sumDP2 := sum.DataPoints().AppendEmpty()
	sumDP2.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	sumDP2.SetIntValue(10)
	sumDP2.Attributes().PutStr("k2", "v")
	sumDP3 := sum.DataPoints().AppendEmpty()
	sumDP3.SetTimestamp(pcommon.NewTimestampFromTime(timestamp1))
	sumDP3.SetDoubleValue(11.12)
	sumDP3.Attributes().PutStr("k", "v2")

	histogram := appendMetric("histogram_metric").SetEmptyHistogram()
	histogramDP := histogram.DataPoints().AppendEmpty()
	histogramDP.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	histogramDP.BucketCounts().Append(1, 1, 2, 3)
	histogramDP.ExplicitBounds().Append(-1.0, 2.0, 3.5)

	summary := appendMetric("summary_metric").SetEmptySummary()
	summaryDP := summary.DataPoints().AppendEmpty()
	summaryDP.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	summaryDP.SetCount(10)
	summaryDP.SetSum(123.456)
	summaryDP.QuantileValues().AppendEmpty() // quantiles are not stored

	invalidHistogram := appendMetric("invalid_histogram_metric").SetEmptyHistogram()
	invalidHistogramDP := invalidHistogram.DataPoints().AppendEmpty()
	invalidHistogramDP.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	// should be one more bucket count than bounds
	invalidHistogramDP.BucketCounts().Append(1, 2, 3)
	invalidHistogramDP.ExplicitBounds().Append(1, 2, 3)
	expectDropped++

	invalidHistogram = appendMetric("invalid_histogram_metric2").SetEmptyHistogram()
	invalidHistogramDP = invalidHistogram.DataPoints().AppendEmpty()
	invalidHistogramDP.SetTimestamp(pcommon.NewTimestampFromTime(timestamp0))
	invalidHistogramDP.BucketCounts().Append(1)
	invalidHistogramDP.ExplicitBounds().Append( /* should be non-empty */ )
	expectDropped++

	events, stats := transformMetrics(t, metrics)
	assert.Equal(t, expectDropped, stats.UnsupportedMetricsDropped)

	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	expected := []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp0,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{Name: "gauge_metric", Value: 1, Type: "gauge"},
				{Name: "sum_metric", Value: 7, Type: "counter"},
				{
					Name: "histogram_metric",
					Type: "histogram",
					Histogram: model.Histogram{
						Counts: []int64{1, 1, 2, 3},
						Values: []float64{-1, 0.5, 2.75, 3.5},
					},
				},
				{
					Name: "summary_metric",
					Type: "summary",
					SummaryMetric: model.SummaryMetric{
						Count: 10,
						Sum:   123.456,
					},
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{Name: "gauge_metric", Value: 4, Type: "gauge"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"k": {Value: "v"}},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{Name: "gauge_metric", Value: 2.3, Type: "gauge"},
				{Name: "sum_metric", Value: 8.9, Type: "counter"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"k": {Value: "v2"}},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{Name: "gauge_metric", Value: 5.6, Type: "gauge"},
				{Name: "sum_metric", Value: 11.12, Type: "counter"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"k2": {Value: "v"}},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{Name: "sum_metric", Value: 10, Type: "counter"},
			},
		},
	}}

	eventsMatch(t, expected, events)
}

func TestConsumeMetricsNaN(t *testing.T) {
	timestamp := time.Unix(123, 0).UTC()
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metricSlice := scopeMetrics.Metrics()

	for _, value := range []float64{math.NaN(), math.Inf(-1), math.Inf(1)} {
		metric := metricSlice.AppendEmpty()
		metric.SetName("gauge")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetDoubleValue(value)
	}

	events, stats := transformMetrics(t, metrics)
	assert.Equal(t, int64(3), stats.UnsupportedMetricsDropped)
	assert.Empty(t, events)
}

func TestConsumeMetricsHostCPU(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metricSlice := scopeMetrics.Metrics()

	timestamp := time.Unix(123, 0).UTC()
	addFloat64Gauge := func(name string, value float64, attributes map[string]interface{}) {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetDoubleValue(value)
		dp.Attributes().FromRaw(attributes)
	}

	addFloat64Gauge("system.cpu.utilization", 0.8, map[string]interface{}{
		"state": "idle",
		"cpu":   "0",
	})
	addFloat64Gauge("system.cpu.utilization", 0.1, map[string]interface{}{
		"state": "system",
		"cpu":   "0",
	})
	addFloat64Gauge("system.cpu.utilization", 0.1, map[string]interface{}{
		"state": "user",
		"cpu":   "0",
	})

	addFloat64Gauge("system.cpu.utilization", 0.45, map[string]interface{}{
		"state": "idle",
		"cpu":   "1",
	})
	addFloat64Gauge("system.cpu.utilization", 0.05, map[string]interface{}{
		"state": "system",
		"cpu":   "1",
	})
	addFloat64Gauge("system.cpu.utilization", 0.5, map[string]interface{}{
		"state": "user",
		"cpu":   "1",
	})

	addFloat64Gauge("system.cpu.utilization", 0.59, map[string]interface{}{
		"state": "idle",
		"cpu":   "2",
	})
	addFloat64Gauge("system.cpu.utilization", 0.01, map[string]interface{}{
		"state": "system",
		"cpu":   "2",
	})
	addFloat64Gauge("system.cpu.utilization", 0.4, map[string]interface{}{
		"state": "user",
		"cpu":   "2",
	})

	addFloat64Gauge("system.cpu.utilization", 0.6, map[string]interface{}{
		"state": "idle",
		"cpu":   "3",
	})
	addFloat64Gauge("system.cpu.utilization", 0.3, map[string]interface{}{
		"state": "system",
		"cpu":   "3",
	})
	addFloat64Gauge("system.cpu.utilization", 0.1, map[string]interface{}{
		"state": "user",
		"cpu":   "3",
	})

	events, _ := transformMetrics(t, metrics)
	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	expected := []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "idle"}, "cpu": {Value: "0"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.8,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "system"}, "cpu": {Value: "0"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.1,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "user"}, "cpu": {Value: "0"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.1,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "idle"}, "cpu": {Value: "1"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.45,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "system"}, "cpu": {Value: "1"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.05,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "user"}, "cpu": {Value: "1"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.5,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "idle"}, "cpu": {Value: "2"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.59,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "system"}, "cpu": {Value: "2"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.01,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "user"}, "cpu": {Value: "2"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.4,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "idle"}, "cpu": {Value: "3"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.6,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "system"}, "cpu": {Value: "3"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.3,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "user"}, "cpu": {Value: "3"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.utilization",
					Type:  "gauge",
					Value: 0.1,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.cpu.total.norm.pct",
					Value: 0.39000000000000007,
				},
			},
		},
	}}

	eventsMatch(t, expected, events)
}

func TestConsumeMetricsHostMemory(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metricSlice := scopeMetrics.Metrics()

	timestamp := time.Unix(123, 0).UTC()
	addInt64Sum := func(name string, value int64, attributes map[string]interface{}) {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		sum := metric.SetEmptySum()
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetIntValue(value)
		dp.Attributes().FromRaw(attributes)
	}
	addInt64Sum("system.memory.usage", 4773351424, map[string]interface{}{
		"state": "free",
	})
	addInt64Sum("system.memory.usage", 3563778048, map[string]interface{}{
		"state": "used",
	})
	events, _ := transformMetrics(t, metrics)
	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	expected := []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "free"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.memory.usage",
					Type:  "counter",
					Value: 4773351424,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"state": {Value: "used"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.memory.usage",
					Type:  "counter",
					Value: 3563778048,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "system.memory.actual.free",
					Value: 4773351424,
				},
				{
					Name:  "system.memory.total",
					Value: 8337129472,
				},
			},
		},
	}}

	eventsMatch(t, expected, events)
}

func TestConsumeMetrics_JVM(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metricSlice := scopeMetrics.Metrics()

	timestamp := time.Unix(123, 0).UTC()
	addInt64Sum := func(name string, value int64, attributes map[string]interface{}) {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		sum := metric.SetEmptySum()
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetIntValue(value)
		dp.Attributes().FromRaw(attributes)
	}
	addInt64Gauge := func(name string, value int64, attributes map[string]interface{}) {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		dp.SetIntValue(value)
		dp.Attributes().FromRaw(attributes)
	}
	addInt64Sum("runtime.jvm.gc.time", 9, map[string]interface{}{
		"gc": "G1 Young Generation",
	})
	addInt64Sum("runtime.jvm.gc.count", 2, map[string]interface{}{
		"gc": "G1 Young Generation",
	})
	addInt64Gauge("runtime.jvm.memory.area", 42, map[string]interface{}{
		"area": "heap",
		"type": "used",
	})
	addInt64Gauge("runtime.jvm.memory.area", 24, map[string]interface{}{
		"area": "heap",
		"type": "used",
		"pool": "eden",
	})
	addInt64Gauge("process.runtime.jvm.memory.limit", 20000, map[string]interface{}{
		"type": "heap",
		"pool": "G1 Eden Space",
	})

	events, _ := transformMetrics(t, metrics)
	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	expected := []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"gc": {Value: "G1 Young Generation"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "runtime.jvm.gc.time",
					Type:  "counter",
					Value: 9,
				},
				{
					Name:  "runtime.jvm.gc.count",
					Type:  "counter",
					Value: 2,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"name": {Value: "G1 Young Generation"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "jvm.gc.time",
					Value: 9,
				},
				{
					Name:  "jvm.gc.count",
					Value: 2,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"area": {Value: "heap"}, "type": {Value: "used"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "runtime.jvm.memory.area",
					Type:  "gauge",
					Value: 42,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "jvm.memory.heap.used",
					Value: 42,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"area": {Value: "heap"}, "type": {Value: "used"}, "pool": {Value: "eden"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "runtime.jvm.memory.area",
					Type:  "gauge",
					Value: 24,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"name": {Value: "eden"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "jvm.memory.heap.pool.used",
					Value: 24,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"type": {Value: "heap"}, "pool": {Value: "G1 Eden Space"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "process.runtime.jvm.memory.limit",
					Type:  "gauge",
					Value: 20000,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    model.Labels{"name": {Value: "G1 Eden Space"}},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: []model.MetricsetSample{
				{
					Name:  "jvm.memory.heap.pool.max",
					Value: 20000,
				},
			},
		},
	}}

	eventsMatch(t, expected, events)
}

func TestConsumeMetricsExportTimestamp(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// The actual timestamps will be non-deterministic, as they are adjusted
	// based on the server's clock.
	//
	// Use a large delta so that we can allow for a significant amount of
	// delay in the test environment affecting the timestamp adjustment.
	const timeDelta = time.Hour
	const allowedError = 5 // seconds

	now := time.Now()
	exportTimestamp := now.Add(-timeDelta)
	resourceMetrics.Resource().Attributes().PutInt("telemetry.sdk.elastic_export_timestamp", exportTimestamp.UnixNano())

	// Timestamp relative to the export timestamp.
	dataPointOffset := -time.Second
	exportedDataPointTimestamp := exportTimestamp.Add(dataPointOffset)

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("int_gauge")
	intGauge := metric.SetEmptyGauge()
	dp := intGauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(exportedDataPointTimestamp))
	dp.SetIntValue(1)

	events, _ := transformMetrics(t, metrics)
	require.Len(t, events, 1)
	assert.InDelta(t, now.Add(dataPointOffset).Unix(), events[0].Timestamp.Unix(), allowedError)

	for _, e := range events {
		// telemetry.sdk.elastic_export_timestamp should not be sent as a label.
		assert.Empty(t, e.NumericLabels)
	}
}

func TestMetricsLogging(t *testing.T) {
	for _, level := range []logp.Level{logp.InfoLevel, logp.DebugLevel} {
		t.Run(level.String(), func(t *testing.T) {
			logp.DevelopmentSetup(logp.ToObserverOutput(), logp.WithLevel(level))
			transformMetrics(t, pmetric.NewMetrics())
			logs := logp.ObserverLogs().TakeAll()
			if level == logp.InfoLevel {
				assert.Empty(t, logs)
			} else {
				assert.NotEmpty(t, logs)
			}
		})
	}
}

func transformMetrics(t *testing.T, metrics pmetric.Metrics) ([]model.APMEvent, otel.ConsumerStats) {
	var batches []*model.Batch
	recorder := batchRecorderBatchProcessor(&batches)

	consumer := &otel.Consumer{Processor: recorder}
	err := consumer.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	return *batches[0], consumer.Stats()
}

func eventsMatch(t *testing.T, expected []model.APMEvent, actual []model.APMEvent) {
	t.Helper()
	diff := cmp.Diff(
		expected, actual,
		// Ignore order of events and their metrics. Some other slices
		// have a defined order (e.g. histogram counts/values), so we
		// don't ignore the order of all slices.
		//
		// Comparing string representations is a bit of a hack; ideally
		// we would use like https://github.com/google/go-cmp/issues/67
		cmpopts.SortSlices(func(x, y model.APMEvent) bool {
			return fmt.Sprint(x) < fmt.Sprint(y)
		}),
		cmpopts.SortSlices(func(x, y model.MetricsetSample) bool {
			return fmt.Sprint(x) < fmt.Sprint(y)
		}),
		cmp.Comparer(func(x netip.Addr, y netip.Addr) bool {
			return x == y
		}),
	)
	if diff != "" {
		t.Fatal(diff)
	}
}
