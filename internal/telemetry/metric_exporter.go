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

package telemetry

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
)

// NewMetricExporter initializes a new MetricExporter
func NewMetricExporter(p modelpb.BatchProcessor, opts ...ConfigOption) *MetricExporter {
	cfg := newConfig(opts...)

	return &MetricExporter{
		processor: p,

		metricFilter:        cfg.MetricFilter,
		temporalitySelector: cfg.TemporalitySelector,
		aggregationSelector: cfg.AggregationSelector,
	}
}

// MetricExporter is an OpenTelemetry metric Reader which retrieves metrics,
// filters them and emits them to the specified consumer
type MetricExporter struct {
	processor modelpb.BatchProcessor

	metricFilter        []string
	temporalitySelector metric.TemporalitySelector
	aggregationSelector metric.AggregationSelector
}

// Temporality returns the Temporality to use for an instrument kind.
func (e *MetricExporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return e.temporalitySelector(k)
}

// Aggregation returns the Aggregation to use for an instrument kind.
func (e *MetricExporter) Aggregation(k metric.InstrumentKind) aggregation.Aggregation {
	return e.aggregationSelector(k)
}

func (e *MetricExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	batch := modelpb.Batch{}
	now := time.Now()

	baseEvent := modelpb.APMEvent{
		Service: &modelpb.Service{
			Name:     "apm-server",
			Language: &modelpb.Language{Name: "go"},
		},
		Agent: &modelpb.Agent{
			Name:    "internal",
			Version: "unknown",
		},
		Event: &modelpb.Event{
			Received: modelpb.FromTime(now),
		},
	}

	for _, scopeMetrics := range rm.ScopeMetrics {
		ms := make(metricsets)
		for _, sm := range scopeMetrics.Metrics {
			if e.isMetricFiltered(sm.Name) {
				continue
			}

			if err := addMetric(sm, ms); err != nil {
				return err
			}
		}

		for key, ms := range ms {
			event := baseEvent.CloneVT()
			event.Timestamp = modelpb.FromTime(key.timestamp)
			metrs := make([]*modelpb.MetricsetSample, 0, len(ms.samples))
			for _, s := range ms.samples {
				metrs = append(metrs, s)
			}
			event.Metricset = &modelpb.Metricset{Samples: metrs, Name: "app"}
			if ms.attributes.Len() > 0 {
				event.Labels = modelpb.Labels(event.Labels).Clone()
				event.NumericLabels = modelpb.NumericLabels(event.NumericLabels).Clone()

				iter := ms.attributes.Iter()
				for iter.Next() {
					_, kv := iter.IndexedAttribute()
					setLabel(string(kv.Key), event, kv.Value.Emit())
				}
				if len(event.Labels) == 0 {
					event.Labels = nil
				}
				if len(event.NumericLabels) == 0 {
					event.NumericLabels = nil
				}
			}

			batch = append(batch, event)
		}
	}
	if len(batch) == 0 {
		return nil
	}

	return e.processor.ProcessBatch(ctx, &batch)
}

func (e *MetricExporter) isMetricFiltered(n string) bool {
	for _, v := range e.metricFilter {
		if v == n {
			return false
		}
	}
	return true
}

func addMetric(sm metricdata.Metrics, ms metricsets) error {
	switch m := sm.Data.(type) {
	case metricdata.Histogram[int64]:
		for _, dp := range m.DataPoints {
			if hist := buildHistogram(dp); hist != nil {
				sample := modelpb.MetricsetSample{
					Name:      sm.Name,
					Type:      modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
					Histogram: hist,
				}
				ms.upsert(dp.Time, dp.Attributes, &sample)
			}
		}
	case metricdata.Histogram[float64]:
		for _, dp := range m.DataPoints {
			if hist := buildHistogram(dp); hist != nil {
				sample := modelpb.MetricsetSample{
					Name:      sm.Name,
					Type:      modelpb.MetricType_METRIC_TYPE_HISTOGRAM,
					Histogram: hist,
				}
				ms.upsert(dp.Time, dp.Attributes, &sample)
			}
		}
	case metricdata.Sum[int64]:
		for _, dp := range m.DataPoints {
			sample := modelpb.MetricsetSample{
				Name:  sm.Name,
				Type:  modelpb.MetricType_METRIC_TYPE_COUNTER,
				Value: float64(dp.Value),
			}
			ms.upsert(dp.Time, dp.Attributes, &sample)
		}
	case metricdata.Sum[float64]:
		for _, dp := range m.DataPoints {
			sample := modelpb.MetricsetSample{
				Name:  sm.Name,
				Type:  modelpb.MetricType_METRIC_TYPE_COUNTER,
				Value: dp.Value,
			}
			ms.upsert(dp.Time, dp.Attributes, &sample)
		}
	case metricdata.Gauge[int64]:
		for _, dp := range m.DataPoints {
			sample := modelpb.MetricsetSample{
				Name:  sm.Name,
				Type:  modelpb.MetricType_METRIC_TYPE_GAUGE,
				Value: float64(dp.Value),
			}
			ms.upsert(dp.Time, dp.Attributes, &sample)
		}
	case metricdata.Gauge[float64]:
		for _, dp := range m.DataPoints {
			sample := modelpb.MetricsetSample{
				Name:  sm.Name,
				Type:  modelpb.MetricType_METRIC_TYPE_GAUGE,
				Value: dp.Value,
			}
			ms.upsert(dp.Time, dp.Attributes, &sample)
		}
	default:
		return fmt.Errorf("unknown metric type %q", m)
	}

	return nil
}

func buildHistogram[T int64 | float64](dp metricdata.HistogramDataPoint[T]) *modelpb.Histogram {
	if len(dp.BucketCounts) != len(dp.Bounds)+1 || len(dp.Bounds) == 0 {
		return nil
	}

	bounds := make([]float64, 0, len(dp.Bounds))
	counts := make([]uint64, 0, len(dp.BucketCounts))

	for i, _ := range dp.BucketCounts {
		bound, count := computeCountAndBounds(i, dp.Bounds, dp.BucketCounts)
		if count == 0 {
			continue
		}

		counts = append(counts, count)
		bounds = append(bounds, bound)
	}

	return &modelpb.Histogram{
		Counts: counts,
		Values: bounds,
	}
}

func computeCountAndBounds(i int, bounds []float64, counts []uint64) (float64, uint64) {
	count := counts[i]
	if count == 0 {
		return 0, 0
	}

	var bound float64
	switch i {
	// (-infinity, explicit_bounds[i]]
	case 0:
		bound = bounds[i]
		if bound > 0 {
			bound /= 2
		}

	// (explicit_bounds[i], +infinity)
	case len(counts) - 1:
		bound = bounds[i-1]

	// [explicit_bounds[i-1], explicit_bounds[i])
	default:
		// Use the midpoint between the boundaries.
		bound = bounds[i-1] + (bounds[i]-bounds[i-1])/2.0
	}

	return bound, count
}

func (e *MetricExporter) ForceFlush(ctx context.Context) error {
	return ctx.Err()
}

func (e *MetricExporter) Shutdown(ctx context.Context) error {
	return nil
}

func setLabel(key string, event *modelpb.APMEvent, v interface{}) {
	switch v := v.(type) {
	case string:
		modelpb.Labels(event.Labels).Set(key, v)
	case bool:
		modelpb.Labels(event.Labels).Set(key, strconv.FormatBool(v))
	case float64:
		modelpb.NumericLabels(event.NumericLabels).Set(key, v)
	case int64:
		modelpb.NumericLabels(event.NumericLabels).Set(key, float64(v))
	case []interface{}:
		if len(v) == 0 {
			return
		}
		switch v[0].(type) {
		case string:
			value := make([]string, len(v))
			for i := range v {
				value[i] = v[i].(string)
			}
			modelpb.Labels(event.Labels).SetSlice(key, value)
		case float64:
			value := make([]float64, len(v))
			for i := range v {
				value[i] = v[i].(float64)
			}
			modelpb.NumericLabels(event.NumericLabels).SetSlice(key, value)
		}
	}
}

type metricsets map[metricsetKey]metricset

type metricsetKey struct {
	timestamp time.Time
	signature string // combination of all attributes
}

type metricset struct {
	attributes attribute.Set
	samples    map[string]*modelpb.MetricsetSample
}

// upsert searches for an existing metricset with the given timestamp and labels,
// and appends the sample to it. If there is no such existing metricset, a new one
// is created.
func (ms metricsets) upsert(timestamp time.Time, attributes attribute.Set, sample *modelpb.MetricsetSample) {
	// We always record metrics as they are given. We also copy some
	// well-known OpenTelemetry metrics to their Elastic APM equivalents.
	ms.upsertOne(timestamp, attributes, sample)
}

func (ms metricsets) upsertOne(timestamp time.Time, attributes attribute.Set, sample *modelpb.MetricsetSample) {
	var signatureBuilder strings.Builder
	iter := attributes.Iter()
	for iter.Next() {
		_, kv := iter.IndexedAttribute()
		signatureBuilder.WriteString(string(kv.Key))
		signatureBuilder.WriteString(kv.Value.Emit())
	}

	key := metricsetKey{timestamp: timestamp, signature: signatureBuilder.String()}

	m, ok := ms[key]
	if !ok {
		m = metricset{
			attributes: attributes,
			samples:    make(map[string]*modelpb.MetricsetSample),
		}
		ms[key] = m
	}
	m.samples[sample.Name] = sample
}
