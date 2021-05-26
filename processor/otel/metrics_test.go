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

	metric = appendMetric("double_histogram_metric", pdata.MetricDataTypeDoubleHistogram)
	doubleHistogram := metric.DoubleHistogram()
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

	metric = appendMetric("invalid_histogram_metric", pdata.MetricDataTypeDoubleHistogram)
	invalidHistogram := metric.DoubleHistogram()
	invalidHistogram.DataPoints().Resize(1)
	invalidHistogram.DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timestamp0))
	invalidHistogram.DataPoints().At(0).SetBucketCounts([]uint64{1, 2, 3}) // should be one more bucket count than bounds
	invalidHistogram.DataPoints().At(0).SetExplicitBounds([]float64{1, 2, 3})
	expectDropped++

	// Summary metrics are not yet supported, and will be dropped.
	metric = appendMetric("double_summary_metric", pdata.MetricDataTypeDoubleSummary)
	metric.DoubleSummary().DataPoints().Resize(1)
	expectDropped++

	metadata := model.Metadata{
		Service: model.Service{
			Name: "unknown",
			Language: model.Language{
				Name: "unknown",
			},
			Agent: model.Agent{
				Name:    "otlp",
				Version: "unknown",
			},
		},
	}

	metricsets, stats := transformMetrics(t, metrics)
	assert.Equal(t, expectDropped, stats.UnsupportedMetricsDropped)

	assert.Equal(t, []*model.Metricset{{
		Metadata:  metadata,
		Timestamp: timestamp0,
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 1, Type: "gauge"},
			{Name: "double_gauge_metric", Value: 5, Type: "gauge"},
			{Name: "int_sum_metric", Value: 9, Type: "counter"},
			{Name: "double_sum_metric", Value: 12, Type: "counter"},

			{
				Name: "double_histogram_metric", Type: "histogram",
				Counts: []int64{1, 1, 2, 3},
				Values: []float64{-1, 0.5, 2.75, 3.5},
			},
			{
				Name: "int_histogram_metric", Type: "histogram",
				Counts: []int64{1, 2, 3},
				Values: []float64{1.5, 2.5, 3},
			},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 3, Type: "gauge"},
			{Name: "double_gauge_metric", Value: 7, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v"},
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 2, Type: "gauge"},
			{Name: "double_gauge_metric", Value: 6, Type: "gauge"},
			{Name: "int_sum_metric", Value: 10, Type: "counter"},
			{Name: "double_sum_metric", Value: 13, Type: "counter"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v2"},
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 4, Type: "gauge"},
			{Name: "double_gauge_metric", Value: 8, Type: "gauge"},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k2": "v"},
		Samples: []model.Sample{
			{Name: "int_sum_metric", Value: 11, Type: "counter"},
			{Name: "double_sum_metric", Value: 14, Type: "counter"},
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
		Service: model.Service{
			Name: "unknown",
			Language: model.Language{
				Name: "unknown",
			},
			Agent: model.Agent{
				Name:    "otlp",
				Version: "unknown",
			},
		},
	}

	metricsets, _ := transformMetrics(t, metrics)
	assert.Equal(t, []*model.Metricset{{
		Metadata:  metadata,
		Timestamp: timestamp,
		Samples: []model.Sample{{
			Type:  "gauge",
			Name:  "jvm.memory.heap.used",
			Value: 42,
		}},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"gc": "G1 Young Generation"},
		Samples: []model.Sample{{
			Type:  "counter",
			Name:  "runtime.jvm.gc.time",
			Value: 9,
		}, {
			Type:  "counter",
			Name:  "runtime.jvm.gc.count",
			Value: 2,
		}},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"name": "G1 Young Generation"},
		Samples: []model.Sample{{
			Type:  "counter",
			Name:  "jvm.gc.time",
			Value: 9,
		}, {
			Type:  "counter",
			Name:  "jvm.gc.count",
			Value: 2,
		}},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp,
		Labels:    common.MapStr{"area": "heap", "type": "used"},
		Samples: []model.Sample{{
			Type:  "gauge",
			Name:  "runtime.jvm.memory.area",
			Value: 42,
		}},
	}}, metricsets)
}

func transformMetrics(t *testing.T, metrics pdata.Metrics) ([]*model.Metricset, otel.ConsumerStats) {
	var batches []*model.Batch
	recorder := batchRecorderBatchProcessor(&batches)

	consumer := &otel.Consumer{Processor: recorder}
	err := consumer.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	return batches[0].Metricsets, consumer.Stats()
}
