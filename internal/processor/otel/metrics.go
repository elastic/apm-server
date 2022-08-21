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
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/elastic-agent-libs/logp"
)

// ConsumeMetrics consumes OpenTelemetry metrics data, converting into
// the Elastic APM metrics model and sending to the reporter.
func (c *Consumer) ConsumeMetrics(ctx context.Context, metrics pmetric.Metrics) error {
	receiveTimestamp := time.Now()
	logger := logp.NewLogger(logs.Otel)
	if logger.IsDebug() {
		data, err := jsonMetricsMarshaler.MarshalMetrics(metrics)
		if err != nil {
			logger.Debug(err)
		} else {
			logger.Debug(string(data))
		}
	}
	batch := c.convertMetrics(metrics, receiveTimestamp)
	return c.Processor.ProcessBatch(ctx, batch)
}

func (c *Consumer) convertMetrics(metrics pmetric.Metrics, receiveTimestamp time.Time) *model.Batch {
	batch := model.Batch{}
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		c.convertResourceMetrics(resourceMetrics.At(i), receiveTimestamp, &batch)
	}
	return &batch
}

func (c *Consumer) convertResourceMetrics(resourceMetrics pmetric.ResourceMetrics, receiveTimestamp time.Time, out *model.Batch) {
	var baseEvent model.APMEvent
	var timeDelta time.Duration
	resource := resourceMetrics.Resource()
	translateResourceMetadata(resource, &baseEvent)
	if exportTimestamp, ok := exportTimestamp(resource); ok {
		timeDelta = receiveTimestamp.Sub(exportTimestamp)
	}
	scopeMetrics := resourceMetrics.ScopeMetrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		c.convertScopeMetrics(scopeMetrics.At(i), baseEvent, timeDelta, out)
	}
}

// OTel specification : https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/semantic_conventions/system-metrics.md
type apmMetricsBuilder struct {
	// Host CPU metrics
	cpuCount                 int // from system.cpu.utilization's cpu attribute
	nonIdleCPUUtilizationSum apmMetricValue

	// Host memory metrics
	freeMemoryBytes apmMetricValue
	usedMemoryBytes apmMetricValue

	// JVM metrics
	jvmGCTime  map[string]apmMetricValue
	jvmGCCount map[string]apmMetricValue
	jvmMemory  map[jvmMemoryKey]apmMetricValue
}

func newAPMMetricsBuilder() *apmMetricsBuilder {
	var b apmMetricsBuilder
	b.jvmGCTime = make(map[string]apmMetricValue)
	b.jvmGCCount = make(map[string]apmMetricValue)
	b.jvmMemory = make(map[jvmMemoryKey]apmMetricValue)
	return &b
}

type apmMetricValue struct {
	timestamp time.Time
	value     float64
}

type jvmMemoryKey struct {
	area    string
	jvmType string
	pool    string // will be "" for non-pool specific memory metrics
}

// accumulate processes m, translating to and accumulating equivalent Elastic APM metrics in b.
func (b *apmMetricsBuilder) accumulate(m pmetric.Metric) {

	switch m.DataType() {
	case pmetric.MetricDataTypeSum:
		dpsCounter := m.Sum().DataPoints()
		for i := 0; i < dpsCounter.Len(); i++ {
			dp := dpsCounter.At(i)
			if sample, ok := numberSample(dp, model.MetricTypeCounter); ok {
				switch m.Name() {
				case "system.memory.usage":
					if memoryState, exists := dp.Attributes().Get("state"); exists {
						switch memoryState.StringVal() {
						case "used":
							b.usedMemoryBytes = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
						case "free":
							b.freeMemoryBytes = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
						}
					}
				case "runtime.jvm.gc.collection", "runtime.jvm.gc.time":
					if gcName, exists := dp.Attributes().Get("gc"); exists {
						b.jvmGCTime[gcName.StringVal()] = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
					}
				case "runtime.jvm.gc.count":
					if gcName, exists := dp.Attributes().Get("gc"); exists {
						b.jvmGCCount[gcName.StringVal()] = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
					}
				}
			}
		}
	case pmetric.MetricDataTypeGauge:
		// Gauge metrics accumulation
		dpsGauge := m.Gauge().DataPoints()
		for i := 0; i < dpsGauge.Len(); i++ {
			dp := dpsGauge.At(i)
			if sample, ok := numberSample(dp, model.MetricTypeGauge); ok {
				switch m.Name() {
				case "system.cpu.utilization":
					if cpuState, exists := dp.Attributes().Get("state"); exists {
						if cpuState.StringVal() != "idle" {
							b.nonIdleCPUUtilizationSum.value += sample.Value
							b.nonIdleCPUUtilizationSum.timestamp = dp.Timestamp().AsTime()
						}
					}
					if cpuIDStr, exists := dp.Attributes().Get("cpu"); exists {
						cpuID, _ := strconv.Atoi(cpuIDStr.StringVal())
						if cpuID+1 > b.cpuCount {
							b.cpuCount = cpuID + 1
						}
					}
				case "process.runtime.jvm.memory.usage",
					"process.runtime.jvm.memory.init",
					"process.runtime.jvm.memory.committed",
					"process.runtime.jvm.memory.limit":

					var key jvmMemoryKey
					dp.Attributes().Range(func(k string, v pcommon.Value) bool {
						switch k {
						case "type":
							key.area = v.AsString()
						case "pool":
							key.pool = v.AsString()
						}
						return true
					})

					switch m.Name()[strings.LastIndex(m.Name(), ".")+1:] {
					case "limit":
						key.jvmType = "max"
					case "usage":
						key.jvmType = "used"
					case "committed":
						key.jvmType = "committed"
					}

					if key.jvmType != "" {
						b.jvmMemory[key] = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
					}
				// runtime.jvm.* metrics were renamed in the OTel Java SDK v1.13.0
				// (https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/tag/v1.13.0)
				// We should remove this code some time in the future.
				case "runtime.jvm.memory.area":
					var key jvmMemoryKey
					dp.Attributes().Range(func(k string, v pcommon.Value) bool {
						switch k {
						case "area":
							key.area = v.AsString()
						case "type":
							key.jvmType = v.AsString()
						case "pool":
							key.pool = v.AsString()
						}
						return true
					})
					if key.area != "" && key.jvmType != "" {
						b.jvmMemory[key] = apmMetricValue{dp.Timestamp().AsTime(), sample.Value}
					}
				}
			}
		}
	}
}

// emit upserts Elastic APM metrics into ms from information accumulated in b.
func (b *apmMetricsBuilder) emit(ms metricsets) {
	// system.memory.actual.free
	// Direct translation of system.memory.usage (state = free)
	if b.freeMemoryBytes.value > 0 {
		ms.upsertOne(
			b.freeMemoryBytes.timestamp, "system.memory.actual.free", pcommon.NewMap(),
			model.MetricsetSample{Value: b.freeMemoryBytes.value},
		)
	}
	// system.memory.total
	// system.memory.usage (state = free) + system.memory.usage (state = used)
	totalMemoryBytes := b.freeMemoryBytes.value + b.usedMemoryBytes.value
	if totalMemoryBytes > 0 {
		ms.upsertOne(
			b.freeMemoryBytes.timestamp, "system.memory.total", pcommon.NewMap(),
			model.MetricsetSample{Value: totalMemoryBytes},
		)
	}
	// system.cpu.total.norm.pct
	// Averaging of non-idle CPU utilization over all CPU cores
	if b.nonIdleCPUUtilizationSum.value > 0 && b.cpuCount > 0 {
		ms.upsertOne(
			b.nonIdleCPUUtilizationSum.timestamp, "system.cpu.total.norm.pct", pcommon.NewMap(),
			model.MetricsetSample{Value: b.nonIdleCPUUtilizationSum.value / float64(b.cpuCount)},
		)
	}
	// jvm.gc.time
	// Direct translation of runtime.jvm.gc.time or runtime.jvm.gc.collection
	for k, v := range b.jvmGCTime {
		elasticapmAttributes := pcommon.NewMap()
		elasticapmAttributes.Insert("name", pcommon.NewValueString(k))
		ms.upsertOne(
			v.timestamp, "jvm.gc.time", elasticapmAttributes,
			model.MetricsetSample{Value: v.value},
		)
	}
	// jvm.gc.count
	// Direct translation of runtime.jvm.gc.count
	for k, v := range b.jvmGCCount {
		elasticapmAttributes := pcommon.NewMap()
		elasticapmAttributes.Insert("name", pcommon.NewValueString(k))
		ms.upsertOne(
			v.timestamp, "jvm.gc.count", elasticapmAttributes,
			model.MetricsetSample{Value: v.value},
		)
	}
	// jvm.memory.<area>.<type>
	// Direct translation of runtime.jvm.memory.area (area = xxx, type = xxx)
	for k, v := range b.jvmMemory {
		elasticapmAttributes := pcommon.NewMap()
		var elasticapmMetricName string
		if k.pool != "" {
			elasticapmAttributes.Insert("name", pcommon.NewValueString(k.pool))
			elasticapmMetricName = fmt.Sprintf("jvm.memory.%s.pool.%s", k.area, k.jvmType)
		} else {
			elasticapmMetricName = fmt.Sprintf("jvm.memory.%s.%s", k.area, k.jvmType)
		}
		ms.upsertOne(
			v.timestamp, elasticapmMetricName, elasticapmAttributes,
			model.MetricsetSample{Value: v.value},
		)
	}
}

func (c *Consumer) convertScopeMetrics(
	in pmetric.ScopeMetrics,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	out *model.Batch,
) {
	ms := make(metricsets)
	otelMetrics := in.Metrics()
	var unsupported int64
	builder := newAPMMetricsBuilder()
	for i := 0; i < otelMetrics.Len(); i++ {
		builder.accumulate(otelMetrics.At(i))
		if !c.addMetric(otelMetrics.At(i), ms) {
			unsupported++
		}
	}
	builder.emit(ms)
	for key, ms := range ms {
		event := baseEvent
		event.Processor = model.MetricsetProcessor
		event.Timestamp = key.timestamp.Add(timeDelta)
		event.Metricset = &model.Metricset{Samples: ms.samples}
		if ms.attributes.Len() > 0 {
			initEventLabels(&event)
			ms.attributes.Range(func(k string, v pcommon.Value) bool {
				setLabel(k, &event, ifaceAttributeValue(v))
				return true
			})
			if len(event.Labels) == 0 {
				event.Labels = nil
			}
			if len(event.NumericLabels) == 0 {
				event.NumericLabels = nil
			}
		}
		*out = append(*out, event)
	}
	if unsupported > 0 {
		atomic.AddInt64(&c.stats.unsupportedMetricsDropped, unsupported)
	}
}

func (c *Consumer) addMetric(metric pmetric.Metric, ms metricsets) bool {
	// TODO(axw) support units
	anyDropped := false
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := numberSample(dp, model.MetricTypeGauge); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), dp.Attributes(), sample)
			} else {
				anyDropped = true
			}
		}
		return !anyDropped
	case pmetric.MetricDataTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := numberSample(dp, model.MetricTypeCounter); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), dp.Attributes(), sample)
			} else {
				anyDropped = true
			}
		}
		return !anyDropped
	case pmetric.MetricDataTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := histogramSample(dp.BucketCounts(), dp.ExplicitBounds()); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), dp.Attributes(), sample)
			} else {
				anyDropped = true
			}
		}
	case pmetric.MetricDataTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			sample := summarySample(dp)
			ms.upsert(dp.Timestamp().AsTime(), metric.Name(), dp.Attributes(), sample)
		}
	default:
		// Unsupported metric: report that it has been dropped.
		anyDropped = true
	}
	return !anyDropped
}

func numberSample(dp pmetric.NumberDataPoint, metricType model.MetricType) (model.MetricsetSample, bool) {
	var value float64
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(dp.IntVal())
	case pmetric.NumberDataPointValueTypeDouble:
		value = dp.DoubleVal()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return model.MetricsetSample{}, false
		}
	default:
		return model.MetricsetSample{}, false
	}
	return model.MetricsetSample{
		Type:  metricType,
		Value: value,
	}, true
}

func summarySample(dp pmetric.SummaryDataPoint) model.MetricsetSample {
	return model.MetricsetSample{
		Type: model.MetricTypeSummary,
		SummaryMetric: model.SummaryMetric{
			Count: int64(dp.Count()),
			Sum:   dp.Sum(),
		},
	}
}

func histogramSample(bucketCounts pcommon.ImmutableUInt64Slice, explicitBounds pcommon.ImmutableFloat64Slice) (model.MetricsetSample, bool) {
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
	if bucketCounts.Len() != explicitBounds.Len()+1 || explicitBounds.Len() == 0 {
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
	values := make([]float64, 0, bucketCounts.Len())
	counts := make([]int64, 0, bucketCounts.Len())
	for i := 0; i < bucketCounts.Len(); i++ {
		count := bucketCounts.At(i)
		if count == 0 {
			continue
		}

		var value float64
		switch i {
		// (-infinity, explicit_bounds[i]]
		case 0:
			value = explicitBounds.At(i)
			if value > 0 {
				value /= 2
			}

		// (explicit_bounds[i], +infinity)
		case bucketCounts.Len() - 1:
			value = explicitBounds.At(i - 1)

		// [explicit_bounds[i-1], explicit_bounds[i])
		default:
			// Use the midpoint between the boundaries.
			value = explicitBounds.At(i-1) + (explicitBounds.At(i)-explicitBounds.At(i-1))/2.0
		}

		counts = append(counts, int64(count))
		values = append(values, value)
	}
	return model.MetricsetSample{
		Type: model.MetricTypeHistogram,
		Histogram: model.Histogram{
			Counts: counts,
			Values: values,
		},
	}, true
}

type metricsets map[metricsetKey]metricset

type metricsetKey struct {
	timestamp time.Time
	signature string // combination of all attributes
}

type metricset struct {
	attributes pcommon.Map
	samples    map[string]model.MetricsetSample
}

// upsert searches for an existing metricset with the given timestamp and labels,
// and appends the sample to it. If there is no such existing metricset, a new one
// is created.
func (ms metricsets) upsert(timestamp time.Time, name string, attributes pcommon.Map, sample model.MetricsetSample) {
	// We always record metrics as they are given. We also copy some
	// well-known OpenTelemetry metrics to their Elastic APM equivalents.
	ms.upsertOne(timestamp, name, attributes, sample)
}

func (ms metricsets) upsertOne(timestamp time.Time, name string, attributes pcommon.Map, sample model.MetricsetSample) {
	var signatureBuilder strings.Builder
	attributes.Range(func(k string, v pcommon.Value) bool {
		signatureBuilder.WriteString(k)
		signatureBuilder.WriteString(v.AsString())
		return true
	})
	key := metricsetKey{timestamp: timestamp, signature: signatureBuilder.String()}

	m, ok := ms[key]
	if !ok {
		m = metricset{
			attributes: attributes,
			samples:    make(map[string]model.MetricsetSample),
		}
		ms[key] = m
	}
	m.samples[name] = sample
}
