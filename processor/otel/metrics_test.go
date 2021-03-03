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
	intGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	intGauge.DataPoints().At(0).SetValue(1)
	intGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intGauge.DataPoints().At(1).SetValue(2)
	intGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intGauge.DataPoints().At(2).SetValue(3)
	intGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intGauge.DataPoints().At(3).SetValue(4)
	intGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("double_gauge_metric", pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	doubleGauge.DataPoints().Resize(4)
	doubleGauge.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	doubleGauge.DataPoints().At(0).SetValue(5)
	doubleGauge.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(1).SetValue(6)
	doubleGauge.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleGauge.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(2).SetValue(7)
	doubleGauge.DataPoints().At(3).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleGauge.DataPoints().At(3).SetValue(8)
	doubleGauge.DataPoints().At(3).LabelsMap().InitFromMap(map[string]string{"k": "v2"})

	metric = appendMetric("int_sum_metric", pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.DataPoints().Resize(3)
	intSum.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	intSum.DataPoints().At(0).SetValue(9)
	intSum.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intSum.DataPoints().At(1).SetValue(10)
	intSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	intSum.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	intSum.DataPoints().At(2).SetValue(11)
	intSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	metric = appendMetric("double_sum_metric", pdata.MetricDataTypeDoubleSum)
	doubleSum := metric.DoubleSum()
	doubleSum.DataPoints().Resize(3)
	doubleSum.DataPoints().At(0).SetTimestamp(pdata.TimestampUnixNano(timestamp0.UnixNano()))
	doubleSum.DataPoints().At(0).SetValue(12)
	doubleSum.DataPoints().At(1).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleSum.DataPoints().At(1).SetValue(13)
	doubleSum.DataPoints().At(1).LabelsMap().InitFromMap(map[string]string{"k": "v"})
	doubleSum.DataPoints().At(2).SetTimestamp(pdata.TimestampUnixNano(timestamp1.UnixNano()))
	doubleSum.DataPoints().At(2).SetValue(14)
	doubleSum.DataPoints().At(2).LabelsMap().InitFromMap(map[string]string{"k2": "v"})

	// Histograms are currently not supported, and will be ignored.
	metric = appendMetric("double_histogram_metric", pdata.MetricDataTypeDoubleHistogram)
	metric.DoubleHistogram().DataPoints().Resize(1)
	expectDropped++
	metric = appendMetric("int_histogram_metric", pdata.MetricDataTypeIntHistogram)
	metric.IntHistogram().DataPoints().Resize(1)
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
			{Name: "int_gauge_metric", Value: 1},
			{Name: "double_gauge_metric", Value: 5},
			{Name: "int_sum_metric", Value: 9},
			{Name: "double_sum_metric", Value: 12},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 3},
			{Name: "double_gauge_metric", Value: 7},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v"},
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 2},
			{Name: "double_gauge_metric", Value: 6},
			{Name: "int_sum_metric", Value: 10},
			{Name: "double_sum_metric", Value: 13},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k": "v2"},
		Samples: []model.Sample{
			{Name: "int_gauge_metric", Value: 4},
			{Name: "double_gauge_metric", Value: 8},
		},
	}, {
		Metadata:  metadata,
		Timestamp: timestamp1,
		Labels:    common.MapStr{"k2": "v"},
		Samples: []model.Sample{
			{Name: "int_sum_metric", Value: 11},
			{Name: "double_sum_metric", Value: 14},
		},
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
