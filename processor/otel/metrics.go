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

package otel

import (
	"context"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/common"
)

// ConsumeMetrics consumes OpenTelemetry metrics data, converting into
// the Elastic APM metrics model and sending to the reporter.
func (c *Consumer) ConsumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	batch := c.convertMetrics(metrics)
	return c.Reporter(ctx, publish.PendingReq{
		Transformables: batch.Transformables(),
		Trace:          true,
	})
}

func (c *Consumer) convertMetrics(metrics pdata.Metrics) *model.Batch {
	batch := model.Batch{}
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		c.convertResourceMetrics(resourceMetrics.At(i), &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceMetrics(resourceMetrics pdata.ResourceMetrics, out *model.Batch) {
	var metadata model.Metadata
	translateResourceMetadata(resourceMetrics.Resource(), &metadata)
	instrumentationLibraryMetrics := resourceMetrics.InstrumentationLibraryMetrics()
	for i := 0; i < instrumentationLibraryMetrics.Len(); i++ {
		c.convertInstrumentationLibraryMetrics(instrumentationLibraryMetrics.At(i), metadata, out)
	}
}

func (c *Consumer) convertInstrumentationLibraryMetrics(in pdata.InstrumentationLibraryMetrics, metadata model.Metadata, out *model.Batch) {
	var ms metricsets
	otelMetrics := in.Metrics()
	var unsupported int64
	for i := 0; i < otelMetrics.Len(); i++ {
		if !c.addMetric(otelMetrics.At(i), &ms) {
			unsupported++
		}
	}
	for _, m := range ms {
		m.Metadata = metadata
		out.Metricsets = append(out.Metricsets, m.Metricset)
	}
	if unsupported > 0 {
		atomic.AddInt64(&c.stats.unsupportedMetricsDropped, unsupported)
	}
}

func (c *Consumer) addMetric(metric pdata.Metric, ms *metricsets) bool {
	switch metric.DataType() {
	case pdata.MetricDataTypeIntGauge:
		dps := metric.IntGauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				asTime(dp.Timestamp()), dp.LabelsMap(),
				model.Sample{
					Name:  metric.Name(),
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeDoubleGauge:
		dps := metric.DoubleGauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				asTime(dp.Timestamp()), dp.LabelsMap(),
				model.Sample{
					Name:  metric.Name(),
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeIntSum:
		dps := metric.IntSum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				asTime(dp.Timestamp()), dp.LabelsMap(),
				model.Sample{
					Name:  metric.Name(),
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeDoubleSum:
		dps := metric.DoubleSum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				asTime(dp.Timestamp()), dp.LabelsMap(),
				model.Sample{
					Name:  metric.Name(),
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeIntHistogram:
		// TODO(axw) https://github.com/elastic/apm-server/issues/3195
	case pdata.MetricDataTypeDoubleHistogram:
		// TODO(axw) https://github.com/elastic/apm-server/issues/3195
	case pdata.MetricDataTypeDoubleSummary:
		// TODO(axw) https://github.com/elastic/apm-server/issues/3195
		// (Not quite the same issue, but the solution would also enable
		// aggregate metrics, which would be appropriate for summaries.)
	}
	// Unsupported metric: report that it has been dropped.
	return false
}

type metricsets []metricset

type metricset struct {
	*model.Metricset
	labels []stringMapItem // sorted by key
}

type stringMapItem struct {
	key   string
	value string
}

// upsert searches for an existing metricset with the given timestamp and labels,
// and appends the sample to it. If there is no such existing metricset, a new one
// is created.
func (ms *metricsets) upsert(timestamp time.Time, labelMap pdata.StringMap, sample model.Sample) {
	labelMap.Sort()
	labels := make([]stringMapItem, 0, labelMap.Len())
	labelMap.ForEach(func(k, v string) {
		labels = append(labels, stringMapItem{k, v})
	})
	var m *model.Metricset
	i := ms.search(timestamp, labels)
	if i < len(*ms) && compareMetricsets((*ms)[i], timestamp, labels) == 0 {
		m = (*ms)[i].Metricset
	} else {
		m = &model.Metricset{Timestamp: timestamp}
		if len(labels) > 0 {
			m.Labels = make(common.MapStr, len(labels))
			for _, label := range labels {
				m.Labels[label.key] = label.value
			}
		}
		head := (*ms)[:i]
		tail := append([]metricset{{Metricset: m, labels: labels}}, (*ms)[i:]...)
		*ms = append(head, tail...)
	}
	m.Samples = append(m.Samples, sample)
}

func (ms *metricsets) search(timestamp time.Time, labels []stringMapItem) int {
	return sort.Search(len(*ms), func(i int) bool {
		return compareMetricsets((*ms)[i], timestamp, labels) >= 0
	})
}

func compareMetricsets(ms metricset, timestamp time.Time, labels []stringMapItem) int {
	if d := ms.Timestamp.Sub(timestamp); d < 0 {
		return -1
	} else if d > 0 {
		return 1
	}
	if n := len(ms.labels) - len(labels); n < 0 {
		return -1
	} else if n > 0 {
		return 1
	}
	for i, la := range ms.labels {
		lb := labels[i]
		if n := strings.Compare(la.key, lb.key); n != 0 {
			return n
		}
		if n := strings.Compare(la.value, lb.value); n != 0 {
			return n
		}
	}
	return 0
}

func asTime(in pdata.TimestampUnixNano) time.Time {
	return time.Unix(0, int64(in)).UTC()
}
