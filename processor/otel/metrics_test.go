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
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
)

func TestConsumeMetrics(t *testing.T) {
	metrics := pdata.NewMetrics()
	resourceMetrics := pdata.NewResourceMetrics()
	metrics.ResourceMetrics().Append(resourceMetrics)
	instrumentationLibraryMetrics := pdata.NewInstrumentationLibraryMetrics()
	resourceMetrics.InstrumentationLibraryMetrics().Append(instrumentationLibraryMetrics)
	metricSlice := instrumentationLibraryMetrics.Metrics()
	appendMetric := func(name string, dataType pdata.MetricDataType) pdata.Metric {
		n := metricSlice.Len()
		metricSlice.Resize(n + 1)
		metric := metricSlice.At(n)
		metric.SetName(name)
		metric.SetDataType(dataType)
		return metric
	}

	timestamp0 := time.Unix(123, 0).UTC()
	timestamp1 := time.Unix(456, 0).UTC()

	var expectDropped int64

	metric := appendMetric("int_gauge_metric", pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()
	intGauge.DataPoints().Resize(4)
	intGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intGauge.DataPoints().At(0).SetValue(1)
	intGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(1).SetValue(2)
	intGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(2).SetValue(3)
	intGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intGauge.DataPoints().At(3).SetValue(4)
	intGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("double_gauge_metric", pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	doubleGauge.DataPoints().Resize(4)
	doubleGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleGauge.DataPoints().At(0).SetValue(5)
	doubleGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(1).SetValue(6)
	doubleGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(2).SetValue(7)
	doubleGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleGauge.DataPoints().At(3).SetValue(8)
	doubleGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("int_sum_metric", pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.DataPoints().Resize(3)
	intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intSum.DataPoints().At(0).SetValue(9)
	intSum.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSum.DataPoints().At(1).SetValue(10)
	intSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intSum.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	intSum.DataPoints().At(2).SetValue(11)
	intSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("double_sum_metric", pdata.MetricDataTypeDoubleSum)
	doubleSum := metric.DoubleSum()
	doubleSum.DataPoints().Resize(3)
	doubleSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleSum.DataPoints().At(0).SetValue(12)
	doubleSum.DataPoints().At(1).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleSum.DataPoints().At(1).SetValue(13)
	doubleSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleSum.DataPoints().At(2).SetTimestamp(pdata.TimestampFromTime(timestamp1))
	doubleSum.DataPoints().At(2).SetValue(14)
	doubleSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("histogram_metric", pdata.MetricDataTypeHistogram)
	doubleHistogram := metric.Histogram()
	doubleHistogram.DataPoints().Resize(1)
	doubleHistogram.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	doubleHistogram.DataPoints().At(0).SetBucketCounts([]uint64{1, 1, 2, 3})
	doubleHistogram.DataPoints().At(0).SetExplicitBounds([]float64{-1.0, 2.0, 3.5})

	metric = appendMetric("int_histogram_metric", pdata.MetricDataTypeIntHistogram)
	intHistogram := metric.IntHistogram()
	intHistogram.DataPoints().Resize(1)
	intHistogram.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	intHistogram.DataPoints().At(0).SetBucketCounts([]uint64{0, 1, 2, 3})
	intHistogram.DataPoints().At(0).SetExplicitBounds([]float64{1.0, 2.0, 3.0})

	metric = appendMetric("invalid_histogram_metric", pdata.MetricDataTypeHistogram)
	invalidHistogram := metric.Histogram()
	invalidHistogram.DataPoints().Resize(1)
	invalidHistogram.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	invalidHistogram.DataPoints().At(0).SetBucketCounts([]uint64{1, 2, 3}) // should be one more bucket count than bounds
	invalidHistogram.DataPoints().At(0).SetExplicitBounds([]float64{1, 2, 3})
	expectDropped++

	// Summary metrics are not yet supported, and will be dropped.
	metric = appendMetric("double_summary_metric", pdata.MetricDataTypeSummary)
	metric.Summary().DataPoints().Resize(1)
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
			"int_gauge_metric":    {Value: 1, Type: "gauge"},
			"double_gauge_metric": {Value: 5, Type: "gauge"},
			"int_sum_metric":      {Value: 9, Type: "counter"},
			"double_sum_metric":   {Value: 12, Type: "counter"},
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
			"int_gauge_metric":    {Value: 3, Type: "gauge"},
			"double_gauge_metric": {Value: 7, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v"},
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric":    {Value: 2, Type: "gauge"},
			"double_gauge_metric": {Value: 6, Type: "gauge"},
			"int_sum_metric":      {Value: 10, Type: "counter"},
			"double_sum_metric":   {Value: 13, Type: "counter"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v2"},
		Samples: map[string]model.MetricsetSample{
			"int_gauge_metric":    {Value: 4, Type: "gauge"},
			"double_gauge_metric": {Value: 8, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k2": "v"},
		Samples: map[string]model.MetricsetSample{
			"int_sum_metric":    {Value: 11, Type: "counter"},
			"double_sum_metric": {Value: 14, Type: "counter"},
		},
	}}, metricsets)
}

func TestConsumeMetrics_JVM(t *testing.T) {
	metrics := pdata.NewMetrics()
	resourceMetrics := pdata.NewResourceMetrics()
	metrics.ResourceMetrics().Append(resourceMetrics)
	instrumentationLibraryMetrics := pdata.NewInstrumentationLibraryMetrics()
	resourceMetrics.InstrumentationLibraryMetrics().Append(instrumentationLibraryMetrics)
	metricSlice := instrumentationLibraryMetrics.Metrics()
	appendMetric := func(name string, dataType pdata.MetricDataType) pdata.Metric {
		n := metricSlice.Len()
		metricSlice.Resize(n + 1)
		metric := metricSlice.At(n)
		metric.SetName(name)
		metric.SetDataType(dataType)
		return metric
	}

	timestamp := time.Unix(123, 0).UTC()
	addInt64Sum := func(name string, value int64, labels map[string]string) {
		metric := appendMetric(name, pdata.MetricDataTypeIntSum)
		intSum := metric.IntSum()
		intSum.DataPoints().Resize(1)
		intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp))
		intSum.DataPoints().At(0).SetValue(value)
		intSum.DataPoints().At(0).LabelsMap().InitFromMap(labels)
	}
	addInt64Gauge := func(name string, value int64, labels map[string]string) {
		metric := appendMetric(name, pdata.MetricDataTypeIntGauge)
		intSum := metric.IntGauge()
		intSum.DataPoints().Resize(1)
		intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp))
		intSum.DataPoints().At(0).SetValue(value)
		intSum.DataPoints().At(0).LabelsMap().InitFromMap(labels)
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
	resourceMetrics := pdata.NewResourceMetrics()
	metrics := pdata.NewMetrics()
	metrics.ResourceMetrics().Append(resourceMetrics)

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

	instrumentationLibraryMetrics := pdata.NewInstrumentationLibraryMetrics()
	resourceMetrics.InstrumentationLibraryMetrics().Append(instrumentationLibraryMetrics)

	metric := pdata.NewMetric()
	metric.SetName("int_gauge")
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()
	intGauge.DataPoints().Resize(1)
	intGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(exportedDataPointTimestamp))
	intGauge.DataPoints().At(0).SetValue(1)
	instrumentationLibraryMetrics.Metrics().Append(metric)

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
