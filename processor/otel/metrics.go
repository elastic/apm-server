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
			logger.Debug(string(data))
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

// This builder groups system metrics with the same name in a map entry, allowing to perform aggregation
// Otel specification : https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/semantic_conventions/system-metrics.md
// ex : total memory = system.memory.usage (state = free) + system.memory.usage (state = used)
type apmMetricBuilder struct {
	metricList []pdata.Metric
	metricName string
}

func (b *apmMetricBuilder) build(ms metricsets) {
	switch b.metricName {
	// Compute and upsert system.memory.total
	case "system.memory.usage":
		var freeDp pdata.NumberDataPoint
		var freeSample model.MetricsetSample
		var usedDp pdata.NumberDataPoint
		var usedSample model.MetricsetSample
		for _, metric := range b.metricList {
			dps := metric.Sum().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				if sample, ok := numberSample(dp, model.MetricTypeCounter); ok {
					dp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
						if k == "state" {
							switch v.StringVal() {
							case "used":
								usedDp = dp
								usedSample = sample
							case "free":
								freeDp = dp
								freeSample = sample
							}
						}
						return true
					})
				}
			}
		}
		if freeDp != (pdata.NumberDataPoint{}) && usedDp != (pdata.NumberDataPoint{}) {
			ms.upsertOne(
				freeDp.Timestamp().AsTime(),
				"system.memory.total",
				pdata.NewAttributeMap(),
				model.MetricsetSample{Type: model.MetricTypeCounter, Value: freeSample.Value + usedSample.Value},
			)
		}
	// Compute system.cpu.total.norm.pct
	// Sum all non-idle utilization metrics	and average over the number of cores
	case "system.cpu.utilization":
		activeProp := float64(0)
		numberCpus := 1
		var bufferDp pdata.NumberDataPoint
		for _, metric := range b.metricList {
			dps := metric.Gauge().DataPoints()
			for i := 0; i < dps.Len(); i++ {
				dp := dps.At(i)
				bufferDp = dp
				if sample, ok := numberSample(dp, model.MetricTypeCounter); ok {
					dp.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
						if k == "state" && v.StringVal() != "idle" {
							if sample.Value > 1 {
								activeProp += 1
							} else {
								activeProp += sample.Value
							}
						}
						if k == "cpu" {
							cpuId, _ := strconv.Atoi(v.StringVal())
							if cpuId+1 > numberCpus {
								numberCpus = cpuId + 1
							}
						}
						return true
					})
				}
			}
		}
		ms.upsertOne(
			bufferDp.Timestamp().AsTime(),
			"system.cpu.total.norm.pct",
			pdata.NewAttributeMap(),
			model.MetricsetSample{Type: model.MetricTypeGauge, Value: activeProp / float64(numberCpus)},
		)
	}
}

func (c *Consumer) convertInstrumentationLibraryMetrics(
	in pdata.InstrumentationLibraryMetrics,
	baseEvent model.APMEvent,
	timeDelta time.Duration,
	out *model.Batch,
) {
	ms := make(metricsets)
	otelMetrics := in.Metrics()
	var unsupported int64
	apmMetricBuilderTracker := make(map[string]*apmMetricBuilder)
	for i := 0; i < otelMetrics.Len(); i++ {
		currentBuilder, exists := apmMetricBuilderTracker[otelMetrics.At(i).Name()]
		if exists {
			currentBuilder.metricList = append(currentBuilder.metricList, otelMetrics.At(i))
		} else {
			currentBuilder := apmMetricBuilder{metricList: make([]pdata.Metric, 0), metricName: otelMetrics.At(i).Name()}
			currentBuilder.metricList = append(currentBuilder.metricList, otelMetrics.At(i))
			apmMetricBuilderTracker[otelMetrics.At(i).Name()] = &currentBuilder
		}
		if !c.addMetric(otelMetrics.At(i), ms) {
			unsupported++
		}
	}
	for key := range apmMetricBuilderTracker {
		apmMetricBuilderTracker[key].build(ms)
	}
	for key, ms := range ms {
		event := baseEvent
		event.Processor = model.MetricsetProcessor
		event.Timestamp = key.timestamp.Add(timeDelta)
		event.Metricset = &model.Metricset{Samples: ms.samples}
		if ms.attributes.Len() > 0 {
			initEventLabels(&event)
			ms.attributes.Range(func(k string, v pdata.AttributeValue) bool {
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

func (c *Consumer) addMetric(metric pdata.Metric, ms metricsets) bool {
	// TODO(axw) support units
	anyDropped := false
	switch metric.DataType() {
	case pdata.MetricDataTypeGauge:
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
	case pdata.MetricDataTypeSum:
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
	case pdata.MetricDataTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			if sample, ok := histogramSample(dp.BucketCounts(), dp.ExplicitBounds()); ok {
				ms.upsert(dp.Timestamp().AsTime(), metric.Name(), dp.Attributes(), sample)
			} else {
				anyDropped = true
			}
		}
	case pdata.MetricDataTypeSummary:
		// TODO(axw) https://github.com/elastic/apm-server/issues/3195
		// (Not quite the same issue, but the solution would also enable
		// aggregate metrics, which would be appropriate for summaries.)
		fallthrough
	default:
		// Unsupported metric: report that it has been dropped.
		anyDropped = true
	}
	return !anyDropped
}

func numberSample(dp pdata.NumberDataPoint, metricType model.MetricType) (model.MetricsetSample, bool) {
	var value float64
	switch dp.Type() {
	case pdata.MetricValueTypeInt:
		value = float64(dp.IntVal())
	case pdata.MetricValueTypeDouble:
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
	attributes pdata.AttributeMap
	samples    map[string]model.MetricsetSample
}

// upsert searches for an existing metricset with the given timestamp and labels,
// and appends the sample to it. If there is no such existing metricset, a new one
// is created.
func (ms metricsets) upsert(timestamp time.Time, name string, attributes pdata.AttributeMap, sample model.MetricsetSample) {
	// We always record metrics as they are given. We also copy some
	// well-known OpenTelemetry metrics to their Elastic APM equivalents.
	ms.upsertOne(timestamp, name, attributes, sample)

	switch name {
	case "runtime.jvm.memory.area":
		// runtime.jvm.memory.area -> jvm.memory.{area}.{type}
		// Copy label "gc" to "name".
		var areaValue, typeValue string
		attributes.Range(func(k string, v pdata.AttributeValue) bool {
			switch k {
			case "area":
				areaValue = v.AsString()
			case "type":
				typeValue = v.AsString()
			}
			return true
		})
		if areaValue != "" && typeValue != "" {
			elasticapmName := fmt.Sprintf("jvm.memory.%s.%s", areaValue, typeValue)
			ms.upsertOne(timestamp, elasticapmName, pdata.NewAttributeMap(), sample)
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
		elasticapmAttributes := pdata.NewAttributeMap()
		attributes.Range(func(k string, v pdata.AttributeValue) bool {
			if k == "gc" {
				elasticapmAttributes.Insert("name", v)
				return false
			}
			return true
		})
		ms.upsertOne(timestamp, elasticapmName, elasticapmAttributes, sample)
	case "system.memory.usage":
		// Translation of Otel memory metrics
		// system.memory.usage (state=free) -> system.memory.actual.free
		// system.memory.usage (state=used) -> system.memory.actual.used.bytes
		var elasticapmName string
		attributes.Range(func(k string, v pdata.AttributeValue) bool {
			if k == "state" {
				switch v.StringVal() {
				case "used":
					elasticapmName = "system.memory.actual.used.bytes"
				case "free":
					elasticapmName = "system.memory.actual.free"
				}
			}
			return true
		})
		ms.upsertOne(timestamp, elasticapmName, pdata.NewAttributeMap(), sample)
	}
}

func (ms metricsets) upsertOne(timestamp time.Time, name string, attributes pdata.AttributeMap, sample model.MetricsetSample) {
	var signatureBuilder strings.Builder
	attributes.Range(func(k string, v pdata.AttributeValue) bool {
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
