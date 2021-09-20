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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
)

func TestConsumeMetrics(t *testing.T) {
	metrics := pdata.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	instrumentationLibraryMetrics := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty()
	metricSlice := instrumentationLibraryMetrics.Metrics()
	appendMetric := func(name string, dataType pdata.MetricDataType) pdata.Metric {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		metric.SetDataType(dataType)
		return metric
	}

	timestamp0 := time.Unix(123, 0).UTC()
	timestamp1 := time.Unix(456, 0).UTC()

	var expectDropped int64

	metric := appendMetric("gauge_metric", pdata.MetricDataTypeGauge)
	gauge := metric.Gauge()
	gaugeDP0 := gauge.DataPoints().AppendEmpty()
	gaugeDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	gaugeDP0.SetIntVal(1)
	gaugeDP1 := gauge.DataPoints().AppendEmpty()
	gaugeDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP1.SetDoubleVal(2.3)
	gaugeDP1.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"k": pdata.NewAttributeValueString("v"),
	})
	gaugeDP2 := gauge.DataPoints().AppendEmpty()
	gaugeDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP2.SetIntVal(4)
	gaugeDP3 := gauge.DataPoints().AppendEmpty()
	gaugeDP3.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP3.SetDoubleVal(5.6)
	gaugeDP3.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"k": pdata.NewAttributeValueString("v2"),
	})

	metric = appendMetric("sum_metric", pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sumDP0 := sum.DataPoints().AppendEmpty()
	sumDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	sumDP0.SetIntVal(7)
	sumDP1 := sum.DataPoints().AppendEmpty()
	sumDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	sumDP1.SetDoubleVal(8.9)
	sumDP1.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"k": pdata.NewAttributeValueString("v"),
	})
	sumDP2 := sum.DataPoints().AppendEmpty()
	sumDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	sumDP2.SetIntVal(10)
	sumDP2.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"k2": pdata.NewAttributeValueString("v"),
	})
	sumDP3 := sum.DataPoints().AppendEmpty()
	sumDP3.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	sumDP3.SetDoubleVal(11.12)
	sumDP3.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"k": pdata.NewAttributeValueString("v2"),
	})

	metric = appendMetric("histogram_metric", pdata.MetricDataTypeHistogram)
	histogram := metric.Histogram()
	histogramDP := histogram.DataPoints().AppendEmpty()
	histogramDP.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	histogramDP.SetBucketCounts([]uint64{1, 1, 2, 3})
	histogramDP.SetExplicitBounds([]float64{-1.0, 2.0, 3.5})

	metric = appendMetric("invalid_histogram_metric", pdata.MetricDataTypeHistogram)
	invalidHistogram := metric.Histogram()
	invalidHistogramDP := invalidHistogram.DataPoints().AppendEmpty()
	invalidHistogramDP.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	invalidHistogramDP.SetBucketCounts([]uint64{1, 2, 3}) // should be one more bucket count than bounds
	invalidHistogramDP.SetExplicitBounds([]float64{1, 2, 3})
	expectDropped++

	// Summary metrics are not yet supported, and will be dropped.
	metric = appendMetric("summary_metric", pdata.MetricDataTypeSummary)
	metric.Summary().DataPoints().AppendEmpty()
	expectDropped++

	events, stats := transformMetrics(t, metrics)
	assert.Equal(t, expectDropped, stats.UnsupportedMetricsDropped)

	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	assert.ElementsMatch(t, []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp0,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"gauge_metric": {Value: 1, Type: "gauge"},
				"sum_metric":   {Value: 7, Type: "counter"},
				"histogram_metric": {
					Type: "histogram",
					Histogram: model.Histogram{
						Counts: []int64{1, 1, 2, 3},
						Values: []float64{-1, 0.5, 2.75, 3.5},
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
			Samples: map[string]model.MetricsetSample{
				"gauge_metric": {Value: 4, Type: "gauge"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"k": "v"},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"gauge_metric": {Value: 2.3, Type: "gauge"},
				"sum_metric":   {Value: 8.9, Type: "counter"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"k": "v2"},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"gauge_metric": {Value: 5.6, Type: "gauge"},
				"sum_metric":   {Value: 11.12, Type: "counter"},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"k2": "v"},
		Timestamp: timestamp1,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"sum_metric": {Value: 10, Type: "counter"},
			},
		},
	}}, events)
}

func TestConsumeMetrics_JVM(t *testing.T) {
	metrics := pdata.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	instrumentationLibraryMetrics := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty()
	metricSlice := instrumentationLibraryMetrics.Metrics()
	appendMetric := func(name string, dataType pdata.MetricDataType) pdata.Metric {
		metric := metricSlice.AppendEmpty()
		metric.SetName(name)
		metric.SetDataType(dataType)
		return metric
	}

	timestamp := time.Unix(123, 0).UTC()
	addInt64Sum := func(name string, value int64, attributes map[string]pdata.AttributeValue) {
		metric := appendMetric(name, pdata.MetricDataTypeSum)
		sum := metric.Sum()
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pdata.TimestampFromTime(timestamp))
		dp.SetIntVal(value)
		dp.Attributes().InitFromMap(attributes)
	}
	addInt64Gauge := func(name string, value int64, attributes map[string]pdata.AttributeValue) {
		metric := appendMetric(name, pdata.MetricDataTypeGauge)
		sum := metric.Gauge()
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pdata.TimestampFromTime(timestamp))
		dp.SetIntVal(value)
		dp.Attributes().InitFromMap(attributes)
	}
	addInt64Sum("runtime.jvm.gc.time", 9, map[string]pdata.AttributeValue{
		"gc": pdata.NewAttributeValueString("G1 Young Generation"),
	})
	addInt64Sum("runtime.jvm.gc.count", 2, map[string]pdata.AttributeValue{
		"gc": pdata.NewAttributeValueString("G1 Young Generation"),
	})
	addInt64Gauge("runtime.jvm.memory.area", 42, map[string]pdata.AttributeValue{
		"area": pdata.NewAttributeValueString("heap"),
		"type": pdata.NewAttributeValueString("used"),
	})

	events, _ := transformMetrics(t, metrics)
	service := model.Service{Name: "unknown", Language: model.Language{Name: "unknown"}}
	agent := model.Agent{Name: "otlp", Version: "unknown"}
	assert.ElementsMatch(t, []model.APMEvent{{
		Agent:     agent,
		Service:   service,
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"jvm.memory.heap.used": {
					Type:  "gauge",
					Value: 42,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"gc": "G1 Young Generation"},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"runtime.jvm.gc.time": {
					Type:  "counter",
					Value: 9,
				},
				"runtime.jvm.gc.count": {
					Type:  "counter",
					Value: 2,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"name": "G1 Young Generation"},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"jvm.gc.time": {
					Type:  "counter",
					Value: 9,
				},
				"jvm.gc.count": {
					Type:  "counter",
					Value: 2,
				},
			},
		},
	}, {
		Agent:     agent,
		Service:   service,
		Labels:    common.MapStr{"area": "heap", "type": "used"},
		Timestamp: timestamp,
		Processor: model.MetricsetProcessor,
		Metricset: &model.Metricset{
			Samples: map[string]model.MetricsetSample{
				"runtime.jvm.memory.area": {
					Type:  "gauge",
					Value: 42,
				},
			},
		},
	}}, events)
}

func TestConsumeMetricsExportTimestamp(t *testing.T) {
	metrics := pdata.NewMetrics()
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
	resourceMetrics.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"telemetry.sdk.elastic_export_timestamp": pdata.NewAttributeValueInt(exportTimestamp.UnixNano()),
	})

	// Timestamp relative to the export timestamp.
	dataPointOffset := -time.Second
	exportedDataPointTimestamp := exportTimestamp.Add(dataPointOffset)

	instrumentationLibraryMetrics := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty()
	metric := instrumentationLibraryMetrics.Metrics().AppendEmpty()
	metric.SetName("int_gauge")
	metric.SetDataType(pdata.MetricDataTypeGauge)
	intGauge := metric.Gauge()
	dp := intGauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pdata.TimestampFromTime(exportedDataPointTimestamp))
	dp.SetIntVal(1)

	events, _ := transformMetrics(t, metrics)
	require.Len(t, events, 1)
	assert.InDelta(t, now.Add(dataPointOffset).Unix(), events[0].Timestamp.Unix(), allowedError)
}

func TestMetricsLogging(t *testing.T) {
	for _, level := range []logp.Level{logp.InfoLevel, logp.DebugLevel} {
		t.Run(level.String(), func(t *testing.T) {
			logp.DevelopmentSetup(logp.ToObserverOutput(), logp.WithLevel(level))
			transformMetrics(t, pdata.NewMetrics())
			logs := logp.ObserverLogs().TakeAll()
			if level == logp.InfoLevel {
				assert.Empty(t, logs)
			} else {
				assert.NotEmpty(t, logs)
			}
		})
	}
}

func transformMetrics(t *testing.T, metrics pdata.Metrics) ([]model.APMEvent, otel.ConsumerStats) {
	var batches []*model.Batch
	recorder := batchRecorderBatchProcessor(&batches)

	consumer := &otel.Consumer{Processor: recorder}
	err := consumer.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	return *batches[0], consumer.Stats()
}
