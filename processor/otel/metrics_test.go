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

	metric := appendMetric("int_gauge_metric", pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()
	intGaugeDP0 := intGauge.DataPoints().AppendEmpty()
	intGaugeDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intGaugeDP0.SetValue(1)
	intGaugeDP1 := intGauge.DataPoints().AppendEmpty()
	intGaugeDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGaugeDP1.SetValue(2)
	intGaugeDP1.LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intGaugeDP2 := intGauge.DataPoints().AppendEmpty()
	intGaugeDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGaugeDP2.SetValue(3)
	intGaugeDP3 := intGauge.DataPoints().AppendEmpty()
	intGaugeDP3.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGaugeDP3.SetValue(4)
	intGaugeDP3.LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("gauge_metric", pdata.MetricDataTypeGauge)
	gauge := metric.Gauge()
	gaugeDP0 := gauge.DataPoints().AppendEmpty()
	gaugeDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	gaugeDP0.SetValue(5)
	gaugeDP1 := gauge.DataPoints().AppendEmpty()
	gaugeDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP1.SetValue(6)
	gaugeDP1.LabelsMap().InitFromMap(map[string]string{"k": "v"})
	gaugeDP2 := gauge.DataPoints().AppendEmpty()
	gaugeDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP2.SetValue(7)
	gaugeDP3 := gauge.DataPoints().AppendEmpty()
	gaugeDP3.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	gaugeDP3.SetValue(8)
	gaugeDP3.LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("int_sum_metric", pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSumDP0 := intSum.DataPoints().AppendEmpty()
	intSumDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intSumDP0.SetValue(9)
	intSumDP1 := intSum.DataPoints().AppendEmpty()
	intSumDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSumDP1.SetValue(10)
	intSumDP1.LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intSumDP2 := intSum.DataPoints().AppendEmpty()
	intSumDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSumDP2.SetValue(11)
	intSumDP2.LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("sum_metric", pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sumDP0 := sum.DataPoints().AppendEmpty()
	sumDP0.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	sumDP0.SetValue(12)
	sumDP1 := sum.DataPoints().AppendEmpty()
	sumDP1.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	sumDP1.SetValue(13)
	sumDP1.LabelsMap().InitFromMap(map[string]string{"k": "v"})
	sumDP2 := sum.DataPoints().AppendEmpty()
	sumDP2.SetTimestamp(pdata.TimestampFromTime(timestamp1))
	sumDP2.SetValue(14)
	sumDP2.LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("histogram_metric", pdata.MetricDataTypeHistogram)
	doubleHistogram := metric.Histogram()
	doubleHistogramDP := doubleHistogram.DataPoints().AppendEmpty()
	doubleHistogramDP.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleHistogramDP.SetBucketCounts([]uint64{1, 1, 2, 3})
	doubleHistogramDP.SetExplicitBounds([]float64{-1.0, 2.0, 3.5})

	metric = appendMetric("int_histogram_metric", pdata.MetricDataTypeIntHistogram)
	intHistogram := metric.IntHistogram()
	intHistogramDP := intHistogram.DataPoints().AppendEmpty()
	intHistogramDP.SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intHistogramDP.SetBucketCounts([]uint64{0, 1, 2, 3})
	intHistogramDP.SetExplicitBounds([]float64{1.0, 2.0, 3.0})

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

	metadata := model.Metadata{
		Agent: model.Agent{
			Name:    "otlp",
			Version: "unknown",
		},
		Service: model.Service{
			Name: "unknown",
			Language: model.Language{
				Name: "unknown",
			},
		},
	}

	metricsets, stats := transformMetrics(t, metrics)
	assert.Equal(t, expectDropped, stats.UnsupportedMetricsDropped)

	assert.Equal(t, []*model.Metricset{{
		Metadata:  metadata,
		Timestamp: timestamp0,
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric": {Value: 1, Type: "gauge"},
			"gauge_metric":     {Value: 5, Type: "gauge"},
			"int_sum_metric":   {Value: 9, Type: "counter"},
			"sum_metric":       {Value: 12, Type: "counter"},
			"histogram_metric": {
				Type:   "histogram",
				Counts: []int64{1, 1, 2, 3},
				Values: []float64{-1, 0.5, 2.75, 3.5},
			},
			"int_histogram_metric": {
				Type:   "histogram",
				Counts: []int64{1, 2, 3},
				Values: []float64{1.5, 2.5, 3},
			},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric": {Value: 3, Type: "gauge"},
			"gauge_metric":     {Value: 7, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v"},
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric": {Value: 2, Type: "gauge"},
			"gauge_metric":     {Value: 6, Type: "gauge"},
			"int_sum_metric":   {Value: 10, Type: "counter"},
			"sum_metric":       {Value: 13, Type: "counter"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v2"},
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric": {Value: 4, Type: "gauge"},
			"gauge_metric":     {Value: 8, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k2": "v"},
		Samples: map[string]model.MetricsetSample{
			"int_sum_metric": {Value: 11, Type: "counter"},
			"sum_metric":     {Value: 14, Type: "counter"},
		},
	}}, metricsets)
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
	addInt64Sum := func(name string, value int64, labels map[string]string) {
		metric := appendMetric(name, pdata.MetricDataTypeIntSum)
		intSum := metric.IntSum()
		dp := intSum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pdata.TimestampFromTime(timestamp))
		dp.SetValue(value)
		dp.LabelsMap().InitFromMap(labels)
	}
	addInt64Gauge := func(name string, value int64, labels map[string]string) {
		metric := appendMetric(name, pdata.MetricDataTypeIntGauge)
		intSum := metric.IntGauge()
		dp := intSum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pdata.TimestampFromTime(timestamp))
		dp.SetValue(value)
		dp.LabelsMap().InitFromMap(labels)
	}
	addInt64Sum("runtime.jvm.gc.time", 9, map[string]string{"gc": "G1 Young Generation"})
	addInt64Sum("runtime.jvm.gc.count", 2, map[string]string{"gc": "G1 Young Generation"})
	addInt64Gauge("runtime.jvm.memory.area", 42, map[string]string{"area": "heap", "type": "used"})

	metadata := model.Metadata{
		Agent: model.Agent{
			Name:    "otlp",
			Version: "unknown",
		},
		Service: model.Service{
			Name: "unknown",
			Language: model.Language{
				Name: "unknown",
			},
		},
	}

	metricsets, _ := transformMetrics(t, metrics)
	assert.Equal(t, []*model.Metricset{{
		Metadata:  metadata,
		Timestamp: timestamp,
		Samples: map[string]model.MetricsetSample{
			"jvm.memory.heap.used": {
				Type:  "gauge",
				Value: 42,
			},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"gc": "G1 Young Generation"},
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
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"name": "G1 Young Generation"},
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
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"area": "heap", "type": "used"},
		Samples: map[string]model.MetricsetSample{
			"runtime.jvm.memory.area": {
				Type:  "gauge",
				Value: 42,
			},
		},
	}}, metricsets)
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
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()
	dp := intGauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pdata.TimestampFromTime(exportedDataPointTimestamp))
	dp.SetValue(1)

	metricsets, _ := transformMetrics(t, metrics)
	require.Len(t, metricsets, 1)
	assert.InDelta(t, now.Add(dataPointOffset).Unix(), metricsets[0].Timestamp.Unix(), allowedError)
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

func transformMetrics(t *testing.T, metrics pdata.Metrics) ([]*model.Metricset, otel.ConsumerStats) {
	var batches []*model.Batch
	recorder := batchRecorderBatchProcessor(&batches)

	consumer := &otel.Consumer{Processor: recorder}
	err := consumer.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	require.Len(t, batches, 1)

	batch := *batches[0]
	metricsets := make([]*model.Metricset, len(batch))
	for i, event := range batch {
		metricsets[i] = event.Metricset
	}
	return metricsets, consumer.Stats()
}
