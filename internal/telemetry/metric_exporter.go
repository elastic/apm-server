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
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
)

type Config struct {
	TemporalitySelector metric.TemporalitySelector
	AggregationSelector metric.AggregationSelector
}
type ConfigOption func(Config) Config

func newConfig(opts ...ConfigOption) Config {
	config := Config{}
	for _, opt := range opts {
		config = opt(config)
	}

	if config.TemporalitySelector == nil {
		config.TemporalitySelector = func(metric.InstrumentKind) metricdata.Temporality {
			return metricdata.CumulativeTemporality
		}
	}
	if config.AggregationSelector == nil {
		config.AggregationSelector = metric.DefaultAggregationSelector
	}

	return config
}

// NewMetricExporter initializes a new MetricExporter
func NewMetricExporter(p modelpb.BatchProcessor, opts ...ConfigOption) *MetricExporter {
	cfg := newConfig(opts...)

	return &MetricExporter{
		processor: p,

		temporalitySelector: cfg.TemporalitySelector,
		aggregationSelector: cfg.AggregationSelector,
	}
}

// MetricExporter is an OpenTelemetry metric Reader which retrieves metrics,
// filters them and emits them to the specified consumer
type MetricExporter struct {
	processor modelpb.BatchProcessor

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

			batch = append(batch, event)
		}
	}

	return e.processor.ProcessBatch(ctx, &batch)
}

func addMetric(sm metricdata.Metrics, ms metricsets) error {
	switch m := sm.Data.(type) {
	case metricdata.Histogram[int64]:
		// Int histogram
	case metricdata.Histogram[float64]:
		// Float Histogram
	case metricdata.Sum[int64]:
		// Int Sum
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
		// Int Gauge
	case metricdata.Gauge[float64]:
		// Float Gauge
	default:
		return fmt.Errorf("unknown metric type %q", m)
	}

	return nil
}

func (e *MetricExporter) ForceFlush(ctx context.Context) error {
	return ctx.Err()
}

func (e *MetricExporter) Shutdown(ctx context.Context) error {
	return nil
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
