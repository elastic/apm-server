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
	"context"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/datastreams"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	metricsetProcessorName  = "metric"
	metricsetDocType        = "metric"
	metricsetEventKey       = "event"
	metricsetTransactionKey = "transaction"
	metricsetSpanKey        = "span"
)

var (
	metricsetMetrics         = monitoring.Default.NewRegistry("apm-server.processor.metric")
	metricsetTransformations = monitoring.NewInt(metricsetMetrics, "transformations")
	metricsetProcessorEntry  = common.MapStr{"name": metricsetProcessorName, "event": metricsetDocType}
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
	Samples []Sample

	// TimeseriesInstanceID holds an optional identifier for the timeseries
	// instance, such as a hash of the labels used for aggregating the
	// metrics.
	TimeseriesInstanceID string
}

// Sample represents a single named metric.
//
// TODO(axw) consider renaming this to "MetricSample" or similar, as
// "Sample" isn't very meaningful in the context of the model package.
type Sample struct {
	// Name holds the metric name.
	Name string

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

func (me *Metricset) Transform(ctx context.Context, _ *transform.Config) []beat.Event {
	metricsetTransformations.Inc()
	if me == nil {
		return nil
	}

	fields := common.MapStr{}
	for _, sample := range me.Samples {
		if err := sample.set(fields); err != nil {
			logp.NewLogger(logs.Transform).Warnf("failed to transform sample %#v", sample)
			continue
		}
	}

	me.Metadata.Set(fields)

	var isInternal bool
	if eventFields := me.Event.fields(); eventFields != nil {
		isInternal = true
		utility.DeepUpdate(fields, metricsetEventKey, eventFields)
	}
	if transactionFields := me.Transaction.fields(); transactionFields != nil {
		isInternal = true
		utility.DeepUpdate(fields, metricsetTransactionKey, transactionFields)
	}
	if spanFields := me.Span.fields(); spanFields != nil {
		isInternal = true
		utility.DeepUpdate(fields, metricsetSpanKey, spanFields)
	}

	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", me.Labels)

	if me.TimeseriesInstanceID != "" {
		fields["timeseries"] = common.MapStr{"instance": me.TimeseriesInstanceID}
	}

	// Metrics are stored in a "metrics" data stream. Metrics that include well-defined
	// transaction/span fields (i.e. breakdown metrics, transaction and span metrics)
	// will be stored separately from application and runtime metrics.
	dataset := datastreams.NormalizeServiceName(me.Metadata.Service.Name)
	if isInternal {
		dataset += ".apm.internal"
	}

	fields["processor"] = metricsetProcessorEntry
	fields[datastreams.TypeField] = datastreams.MetricsType
	fields[datastreams.DatasetField] = dataset

	return []beat.Event{{
		Fields:    fields,
		Timestamp: me.Timestamp,
	}}
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
