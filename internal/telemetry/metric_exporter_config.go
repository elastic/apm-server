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
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
)

// Override default otel/prometheus boundaries, as we skip empty buckets and therefore able to use more accurate and higher range boundaries.
// See https://github.com/elastic/apm/blob/5791498e9569ef9111615ef439e1cbf0b7fd7c18/specs/agents/metrics-otel.md#histogram-aggregation
var customHistogramBoundaries = []float64{
	0.00390625, 0.00552427, 0.0078125, 0.0110485, 0.015625, 0.0220971, 0.03125,
	0.0441942, 0.0625, 0.0883883, 0.125, 0.176777, 0.25, 0.353553, 0.5, 0.707107,
	1, 1.41421, 2, 2.82843, 4, 5.65685, 8, 11.3137, 16, 22.6274, 32, 45.2548, 64,
	90.5097, 128, 181.019, 256, 362.039, 512, 724.077, 1024, 1448.15, 2048,
	2896.31, 4096, 5792.62, 8192, 11585.2, 16384, 23170.5, 32768, 46341.0, 65536,
	92681.9, 131072,
}

type Config struct {
	processor           modelpb.BatchProcessor
	MetricFilter        []string
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
		config.TemporalitySelector = defaultTemporalitySelector
	}
	if config.AggregationSelector == nil {
		config.AggregationSelector = defaultAggregationSelector
	}

	return config
}

func defaultTemporalitySelector(ik metric.InstrumentKind) metricdata.Temporality {
	switch ik {
	case metric.InstrumentKindCounter, metric.InstrumentKindObservableCounter, metric.InstrumentKindHistogram:
		return metricdata.DeltaTemporality
	}
	return metric.DefaultTemporalitySelector(ik)
}

func defaultAggregationSelector(ik metric.InstrumentKind) metric.Aggregation {
	switch ik {
	case metric.InstrumentKindHistogram:
		return metric.AggregationExplicitBucketHistogram{
			Boundaries: customHistogramBoundaries,
			NoMinMax:   false,
		}
	default:
		return metric.DefaultAggregationSelector(ik)
	}
}

// WithBatchProcessor configures the batch processor that will be used by the
// metric exporter.
// Using this option is the equivalent of using `SetBatchProcessor`.
//
// Defaults to not running any batch processing.
func WithBatchProcessor(b modelpb.BatchProcessor) ConfigOption {
	return func(cfg Config) Config {
		cfg.processor = b
		return cfg
	}
}

// WithMetricFilter configured the metrics filter. Any metric filtered here
// will be the only ones to be exported. All other metrics will be ignored.
//
// Defaults to not exporting anything.
func WithMetricFilter(f []string) ConfigOption {
	return func(cfg Config) Config {
		cfg.MetricFilter = f
		return cfg
	}
}

// WithAggregationSelector configure the Aggregation Selector the exporter will
// use. If no AggregationSelector is provided the DefaultAggregationSelector is
// used.
func WithAggregationSelector(agg metric.AggregationSelector) ConfigOption {
	return func(cfg Config) Config {
		cfg.AggregationSelector = agg
		return cfg
	}
}

// WithTemporalitySelector configure the Aggregation Selector the exporter will
// use. If no AggregationSelector is provided the DefaultAggregationSelector is
// used.
func WithTemporalitySelector(temporality metric.TemporalitySelector) ConfigOption {
	return func(cfg Config) Config {
		cfg.TemporalitySelector = temporality
		return cfg
	}
}
