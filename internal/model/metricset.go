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

package model

import (
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

const (
	AppMetricsDataset      = "apm.app"
	InternalMetricsDataset = "apm.internal"
)

var (
	// MetricsetProcessor is the Processor value that should be assigned to metricset events.
	MetricsetProcessor = Processor{Name: "metric", Event: "metric"}
)

// MetricType describes the type of a metric: gauge, counter, or histogram.
type MetricType string

// Valid MetricType values.
const (
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeCounter   MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metricset describes a set of metrics and associated metadata.
type Metricset struct {
	// Samples holds the metrics in the set.
	Samples map[string]MetricsetSample

	// TimeseriesInstanceID holds an optional identifier for the timeseries
	// instance, such as a hash of the labels used for aggregating the
	// metrics.
	TimeseriesInstanceID string

	// Name holds an optional name for the metricset.
	Name string

	// DocCount holds the document count for pre-aggregated metrics.
	//
	// See https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping-doc-count-field.html
	DocCount int64
}

// MetricsetSample represents a single named metric.
type MetricsetSample struct {
	// Type holds an optional metric type.
	//
	// If Type is unspecified or invalid, it will be ignored.
	Type MetricType

	// Unit holds an optional unit:
	//
	// - "percent" (value is in the range [0,1])
	// - "byte"
	// - a time unit: "nanos", "micros", "ms", "s", "m", "h", "d"
	//
	// If Unit is unspecified or invalid, it will be ignored.
	Unit string

	// Value holds the metric value for single-value metrics.
	//
	// If Counts and Values are specified, then Value will be ignored.
	Value float64

	// Histogram holds bucket values and counts for histogram metrics.
	Histogram

	// SummaryMetric holds a combined count and sum of aggregated
	// measurements.
	SummaryMetric
}

// Histogram holds bucket values and counts for a histogram metric.
type Histogram struct {
	// Values holds the bucket values for histogram metrics.
	//
	// These values must be provided in ascending order.
	Values []float64

	// Counts holds the bucket counts for histogram metrics.
	//
	// These numbers must be positive or zero.
	//
	// If Counts is specified, then Values is expected to be
	// specified with the same number of elements, and with the
	// same order.
	Counts []int64
}

func (h *Histogram) fields() mapstr.M {
	if len(h.Counts) == 0 {
		return nil
	}
	var fields mapStr
	fields.set("counts", h.Counts)
	fields.set("values", h.Values)
	return mapstr.M(fields)
}

// SummaryMetric holds summary metrics (count and sum).
type SummaryMetric struct {
	// Count holds the number of aggregated measurements.
	Count int64

	// Sum holds the sum of aggregated measurements.
	Sum float64
}

func (s *SummaryMetric) fields() mapstr.M {
	return mapstr.M{"value_count": s.Count, "sum": s.Sum}
}

// AggregatedDuration holds a count and sum of aggregated durations.
type AggregatedDuration struct {
	// Count holds the number of durations aggregated.
	Count int

	// Sum holds the sum of aggregated durations.
	Sum time.Duration
}

func (a *AggregatedDuration) fields() mapstr.M {
	if a.Count == 0 {
		return nil
	}
	var fields mapStr
	fields.set("count", a.Count)
	fields.set("sum.us", a.Sum.Microseconds())
	return mapstr.M(fields)
}

func (me *Metricset) setFields(fields *mapStr) {
	if me.TimeseriesInstanceID != "" {
		fields.set("timeseries", mapstr.M{"instance": me.TimeseriesInstanceID})
	}
	if me.DocCount > 0 {
		fields.set("_doc_count", me.DocCount)
	}
	fields.maybeSetString("metricset.name", me.Name)

	var metricDescriptions mapStr
	for name, sample := range me.Samples {
		sample.set(name, fields)

		var md mapStr
		md.maybeSetString("type", string(sample.Type))
		md.maybeSetString("unit", sample.Unit)
		metricDescriptions.maybeSetMapStr(name, mapstr.M(md))
	}
	fields.maybeSetMapStr("_metric_descriptions", mapstr.M(metricDescriptions))
}

func (s *MetricsetSample) set(name string, fields *mapStr) {
	switch s.Type {
	case MetricTypeHistogram:
		fields.set(name, s.Histogram.fields())
	case MetricTypeSummary:
		fields.set(name, s.SummaryMetric.fields())
	default:
		fields.set(name, s.Value)
	}
}
