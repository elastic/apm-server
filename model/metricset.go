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
	"fmt"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/datastreams"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/transform"
)

const (
	metricsetProcessorName  = "metric"
	metricsetDocType        = "metric"
	metricsetEventKey       = "event"
	metricsetTransactionKey = "transaction"
	metricsetSpanKey        = "span"
	AppMetricsDataset       = "apm.app"
	InternalMetricsDataset  = "apm.internal"
)

var (
	metricsetMetrics         = monitoring.Default.NewRegistry("apm-server.processor.metric")
	metricsetTransformations = monitoring.NewInt(metricsetMetrics, "transformations")
	metricsetProcessorEntry  = common.MapStr{"name": metricsetProcessorName, "event": metricsetDocType}
)

// MetricType describes the type of a metric: gauge, counter, or histogram.
type MetricType string

// Valid MetricType values.
const (
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeCounter   MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
)

// Metricset describes a set of metrics and associated metadata.
type Metricset struct {
	// Timestamp holds the time at which the metrics were published.
	Timestamp time.Time

	// Metadata holds common metadata describing the entities with which
	// the metrics are associated: service, system, etc.
	Metadata Metadata

	// Event holds information about the event category with which the
	// metrics are associated.
	Event MetricsetEventCategorization

	// Transaction holds information about the transaction group with
	// which the metrics are associated.
	Transaction MetricsetTransaction

	// Span holds information about the span types with which the
	// metrics are associated.
	Span MetricsetSpan

	// Labels holds arbitrary labels to apply to the metrics.
	//
	// These labels override any with the same names in Metadata.Labels.
	Labels common.MapStr

	// Samples holds the metrics in the set.
	//
	// If Samples holds a single histogram metric, then the sum of its Counts
	// will be used to set a _doc_count field in the transformed beat.Event.
	Samples []Sample

	// TimeseriesInstanceID holds an optional identifier for the timeseries
	// instance, such as a hash of the labels used for aggregating the
	// metrics.
	TimeseriesInstanceID string

	// Name holds an optional name for the metricset.
	Name string
}

// Sample represents a single named metric.
//
// TODO(axw) consider renaming this to "MetricSample" or similar, as
// "Sample" isn't very meaningful in the context of the model package.
type Sample struct {
	// Name holds the metric name.
	Name string

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

// MetricsetEventCategorization holds ECS Event Categorization fields
// for inclusion in metrics. Typically these fields will have been
// included in the metric aggregation logic.
//
// See https://www.elastic.co/guide/en/ecs/current/ecs-category-field-values-reference.html
type MetricsetEventCategorization struct {
	// Outcome holds the event outcome: "success", "failure", or "unknown".
	Outcome string
}

// MetricsetTransaction provides enough information to connect a metricset to the related kind of transactions.
type MetricsetTransaction struct {
	// Name holds the transaction name: "GET /foo", etc.
	Name string

	// Type holds the transaction type: "request", "message", etc.
	Type string

	// Result holds the transaction result: "HTTP 2xx", "OK", "Error", etc.
	Result string

	// Root indicates whether or not the transaction is the trace root.
	//
	// If Root is false, then it will be omitted from the output event.
	Root bool
}

// MetricsetSpan provides enough information to connect a metricset to the related kind of spans.
type MetricsetSpan struct {
	// Type holds the span type: "external", "db", etc.
	Type string

	// Subtype holds the span subtype: "http", "sql", etc.
	Subtype string

	// DestinationService holds information about the target of outgoing requests
	DestinationService DestinationService
}

func (me *Metricset) appendBeatEvents(cfg *transform.Config, events []beat.Event) []beat.Event {
	metricsetTransformations.Inc()
	if me == nil {
		return nil
	}

	fields := mapStr{}
	for _, sample := range me.Samples {
		if err := sample.set(common.MapStr(fields)); err != nil {
			logp.NewLogger(logs.Transform).Warnf("failed to transform sample %#v", sample)
			continue
		}
	}
	if len(me.Samples) == 1 && len(me.Samples[0].Counts) > 0 {
		// We have a single histogram metric; add a _doc_count field which holds the sum of counts.
		// See https://www.elastic.co/guide/en/elasticsearch/reference/master/mapping-doc-count-field.html
		var total int64
		for _, count := range me.Samples[0].Counts {
			total += count
		}
		fields["_doc_count"] = total
	}

	me.Metadata.set(&fields, me.Labels)

	var isInternal bool
	if eventFields := me.Event.fields(); eventFields != nil {
		isInternal = true
		common.MapStr(fields).DeepUpdate(common.MapStr{metricsetEventKey: eventFields})
	}
	if transactionFields := me.Transaction.fields(); transactionFields != nil {
		isInternal = true
		common.MapStr(fields).DeepUpdate(common.MapStr{metricsetTransactionKey: transactionFields})
	}
	if spanFields := me.Span.fields(); spanFields != nil {
		isInternal = true
		common.MapStr(fields).DeepUpdate(common.MapStr{metricsetSpanKey: spanFields})
	}

	if me.TimeseriesInstanceID != "" {
		fields["timeseries"] = common.MapStr{"instance": me.TimeseriesInstanceID}
	}

	if me.Name != "" {
		fields["metricset.name"] = me.Name
	}

	fields["processor"] = metricsetProcessorEntry

	// Set a _metric_descriptions field, which holds optional metric types and units.
	var metricDescriptions mapStr
	for _, sample := range me.Samples {
		var m mapStr
		m.maybeSetString("type", string(sample.Type))
		m.maybeSetString("unit", sample.Unit)
		metricDescriptions.maybeSetMapStr(sample.Name, common.MapStr(m))
	}
	fields.maybeSetMapStr("_metric_descriptions", common.MapStr(metricDescriptions))

	if cfg.DataStreams {
		dataset := fmt.Sprintf("%s.%s", AppMetricsDataset, datastreams.NormalizeServiceName(me.Metadata.Service.Name))
		// Metrics are stored in "metrics" data streams.
		if isInternal {
			// Metrics that include well-defined transaction/span fields
			// (i.e. breakdown metrics, transaction and span metrics) will
			// be stored separately from application and runtime metrics.
			dataset = InternalMetricsDataset
		}
		fields[datastreams.DatasetField] = dataset
		fields[datastreams.TypeField] = datastreams.MetricsType
	}

	return append(events, beat.Event{
		Fields:    common.MapStr(fields),
		Timestamp: me.Timestamp,
	})
}

func (e *MetricsetEventCategorization) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("outcome", e.Outcome)
	return common.MapStr(fields)
}

func (t *MetricsetTransaction) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("type", t.Type)
	fields.maybeSetString("name", t.Name)
	fields.maybeSetString("result", t.Result)
	if t.Root {
		fields.set("root", true)
	}
	return common.MapStr(fields)
}

func (s *MetricsetSpan) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("type", s.Type)
	fields.maybeSetString("subtype", s.Subtype)
	if destinationServiceFields := s.DestinationService.fields(); len(destinationServiceFields) != 0 {
		fields.set("destination", common.MapStr{"service": destinationServiceFields})
	}
	return common.MapStr(fields)
}

func (s *Sample) set(fields common.MapStr) error {
	switch {
	case len(s.Counts) > 0:
		_, err := fields.Put(s.Name, common.MapStr{
			"counts": s.Counts,
			"values": s.Values,
		})
		return err
	default:
		_, err := fields.Put(s.Name, s.Value)
		return err
	}
}
