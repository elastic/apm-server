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

	return e.processor.ProcessBatch(ctx, &batch)
}

func (e *MetricExporter) ForceFlush(ctx context.Context) error {
	return ctx.Err()
}

func (e *MetricExporter) Shutdown(ctx context.Context) error {
	return nil
}
