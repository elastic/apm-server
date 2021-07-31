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
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/model/pdata"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/v7/libbeat/logp"
)

// ConsumeMetrics consumes OpenTelemetry metrics data, converting into
// the Elastic APM metrics model and sending to the reporter.
func (c *Consumer) ConsumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	receiveTimestamp := time.Now()
	logger := logp.NewLogger(logs.Otel)
	if logger.IsDebug() {
		data, err := jsonMetricsMarshaler.MarshalMetrics(metrics)
		if err != nil {
			logger.Debug(err)
		} else {
			logger.Debug(data)
		}
	}
	batch := c.convertMetrics(metrics, receiveTimestamp)
	return c.Processor.ProcessBatch(ctx, batch)
}

func (c *Consumer) convertMetrics(metrics pdata.Metrics, receiveTimestamp time.Time) *model.Batch {
	batch := model.Batch{}
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		c.convertResourceMetrics(resourceMetrics.At(i), receiveTimestamp, &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceMetrics(resourceMetrics pdata.ResourceMetrics, receiveTimestamp time.Time, out *model.Batch) {
	var baseEvent model.APMEvent
	var timeDelta time.Duration
	resource := resourceMetrics.Resource()
	translateResourceMetadata(resource, &baseEvent)
	if exportTimestamp, ok := exportTimestamp(resource); ok {
		timeDelta = receiveTimestamp.Sub(exportTimestamp)
	}
	instrumentationLibraryMetrics := resourceMetrics.InstrumentationLibraryMetrics()
	for i := 0; i < instrumentationLibraryMetrics.Len(); i++ {
		c.convertInstrumentationLibraryMetrics(instrumentationLibraryMetrics.At(i), baseEvent, timeDelta, out)
	}
}

func (c *Consumer) convertInstrumentationLibraryMetrics(
	in pdata.InstrumentationLibraryMetrics,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	out *model.Batch,
) {
	var ms metricsets
	otelMetrics := in.Metrics()
	var unsupported int64
	for i := 0; i < otelMetrics.Len(); i++ {
		if !c.addMetric(otelMetrics.At(i), &ms) {
			unsupported++
		}
	}
	for _, m := range ms {
		m.Timestamp = m.Timestamp.Add(timeDelta)
		event := baseEvent
		event.Metricset = m.Metricset
		if n := len(m.labels); n > 0 {
			event.Labels = initEventLabels(event.Labels)
			for _, label := range m.labels {
				event.Labels[label.key] = label.value
			}
		}
		*out = append(*out, event)
	}
	if unsupported > 0 {
		atomic.AddInt64(&c.stats.unsupportedMetricsDropped, unsupported)
	}
}

func (c *Consumer) addMetric(metric pdata.Metric, ms *metricsets) bool {
	// TODO(axw) support units

	switch metric.DataType() {
	case pdata.MetricDataTypeIntGauge:
		dps := metric.IntGauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				dp.Timestamp().AsTime(),
				metric.Name(),
				toStringMapItems(dp.LabelsMap()),
				model.MetricsetSample{
					Type:  model.MetricTypeGauge,
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				dp.Timestamp().AsTime(),
				metric.Name(),
				toStringMapItems(dp.LabelsMap()),
				model.MetricsetSample{
					Type:  model.MetricTypeGauge,
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
				dp.Timestamp().AsTime(),
				metric.Name(),
				toStringMapItems(dp.LabelsMap()),
				model.MetricsetSample{
					Type:  model.MetricTypeCounter,
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			ms.upsert(
				dp.Timestamp().AsTime(),
				metric.Name(),
				toStringMapItems(dp.LabelsMap()),
				model.MetricsetSample{
					Type:  model.MetricTypeCounter,
					Value: float64(dp.Value()),
				},
			)
		}
		return true
	case pdata.MetricDataTypeIntHistogram:
		anyDropped := false
		dps := metric.IntHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := histogramSample(dp.BucketCounts(), dp.ExplicitBounds()); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), toStringMapItems(dp.LabelsMap()), sample)
			} else {
				anyDropped = true
			}
		}
		return !anyDropped
	case pdata.MetricDataTypeHistogram:
		anyDropped := false
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := histogramSample(dp.BucketCounts(), dp.ExplicitBounds()); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), toStringMapItems(dp.LabelsMap()), sample)
			} else {
				anyDropped = true
			}
		}
		return !anyDropped
	case pdata.MetricDataTypeSummary:
		// TODO(axw) https://github.com/elastic/apm-server/issues/3195
		// (Not quite the same issue, but the solution would also enable
		// aggregate metrics, which would be appropriate for summaries.)
	}
	// Unsupported metric: report that it has been dropped.
	return false
}

func histogramSample(bucketCounts []uint64, explicitBounds []float64) (model.MetricsetSample, bool) {
	// (From opentelemetry-proto/opentelemetry/proto/metrics/v1/metrics.proto)
	//
	// This defines size(explicit_bounds) + 1 (= N) buckets. The boundaries for
	// bucket at index i are:
	//
	// (-infinity, explicit_bounds[i]] for i == 0
	// (explicit_bounds[i-1], explicit_bounds[i]] for 0 < i < N-1
	// (explicit_bounds[i], +infinity) for i == N-1
	//
	// The values in the explicit_bounds array must be strictly increasing.
	//
	if len(bucketCounts) != len(explicitBounds)+1 {
		return model.MetricsetSample{}, false
	}

	// For the bucket values, we follow the approach described by Prometheus's
	// histogram_quantile function (https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile)
	// to achieve consistent percentile aggregation results:
	//
	// "The histogram_quantile() function interpolates quantile values by assuming a linear
	// distribution within a bucket. (...) If a quantile is located in the highest bucket,
	// the upper bound of the second highest bucket is returned. A lower limit of the lowest
	// bucket is assumed to be 0 if the upper bound of that bucket is greater than 0. In that
	// case, the usual linear interpolation is applied within that bucket. Otherwise, the upper
	// bound of the lowest bucket is returned for quantiles located in the lowest bucket."
	values := make([]float64, 0, len(bucketCounts))
	counts := make([]int64, 0, len(bucketCounts))
	for i, count := range bucketCounts {
		if count == 0 {
			continue
		}

		var value float64
		switch i {
		// (-infinity, explicit_bounds[i]]
		case 0:
			value = explicitBounds[i]
			if value > 0 {
				value /= 2
			}

		// (explicit_bounds[i], +infinity)
		case len(bucketCounts) - 1:
			value = explicitBounds[i-1]

		// [explicit_bounds[i-1], explicit_bounds[i])
		default:
			// Use the midpoint between the boundaries.
			value = explicitBounds[i-1] + (explicitBounds[i]-explicitBounds[i-1])/2.0
		}

		counts = append(counts, int64(count))
		values = append(values, value)
	}
	return model.MetricsetSample{
		Type:   model.MetricTypeHistogram,
		Counts: counts,
		Values: values,
	}, true
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

func toStringMapItems(labelMap pdata.StringMap) []stringMapItem {
	labels := make([]stringMapItem, 0, labelMap.Len())
	labelMap.Range(func(k, v string) bool {
		labels = append(labels, stringMapItem{k, v})
		return true
	})
	sort.SliceStable(labels, func(i, j int) bool {
		return labels[i].key < labels[j].key
	})
	return labels
}

// upsert searches for an existing metricset with the given timestamp and labels,
// and appends the sample to it. If there is no such existing metricset, a new one
// is created.
func (ms *metricsets) upsert(timestamp time.Time, name string, labels []stringMapItem, sample model.MetricsetSample) {
	// We always record metrics as they are given. We also copy some
	// well-known OpenTelemetry metrics to their Elastic APM equivalents.
	ms.upsertOne(timestamp, name, labels, sample)

	switch name {
	case "runtime.jvm.memory.area":
		// runtime.jvm.memory.area -> jvm.memory.{area}.{type}
		// Copy label "gc" to "name".
		var areaValue, typeValue string
		for _, label := range labels {
			switch label.key {
			case "area":
				areaValue = label.value
			case "type":
				typeValue = label.value
			}
		}
		if areaValue != "" && typeValue != "" {
			elasticapmName := fmt.Sprintf("jvm.memory.%s.%s", areaValue, typeValue)
			ms.upsertOne(timestamp, elasticapmName, nil, sample)
		}
	case "runtime.jvm.gc.collection":
		// This is the old name for runtime.jvm.gc.time.
		name = "runtime.jvm.gc.time"
		fallthrough
	case "runtime.jvm.gc.time", "runtime.jvm.gc.count":
		// Chop off the "runtime." prefix, i.e. runtime.jvm.gc.time -> jvm.gc.time.
		// OpenTelemetry and Elastic APM metrics are both defined in milliseconds.
		elasticapmName := name[len("runtime."):]

		// Copy label "gc" to "name".
		var elasticapmLabels []stringMapItem
		for _, label := range labels {
			if label.key == "gc" {
				elasticapmLabels = []stringMapItem{{key: "name", value: label.value}}
				break
			}
		}
		ms.upsertOne(timestamp, elasticapmName, elasticapmLabels, sample)
	}
}

func (ms *metricsets) upsertOne(timestamp time.Time, name string, labels []stringMapItem, sample model.MetricsetSample) {
	var m *model.Metricset
	i := ms.search(timestamp, labels)
	if i < len(*ms) && compareMetricsets((*ms)[i], timestamp, labels) == 0 {
		m = (*ms)[i].Metricset
	} else {
		m = &model.Metricset{Timestamp: timestamp, Samples: make(map[string]model.MetricsetSample)}
		head := (*ms)[:i]
		tail := append([]metricset{{Metricset: m, labels: labels}}, (*ms)[i:]...)
		*ms = append(head, tail...)
	}
	m.Samples[name] = sample
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
