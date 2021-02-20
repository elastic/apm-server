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
	"fmt"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	transactionProcessorName = "transaction"
	transactionDocType       = "transaction"
	TracesDataset            = "apm"
)

var (
	transactionMetrics         = monitoring.Default.NewRegistry("apm-server.processor.transaction")
	transactionTransformations = monitoring.NewInt(transactionMetrics, "transformations")
	transactionProcessorEntry  = common.MapStr{"name": transactionProcessorName, "event": transactionDocType}
)

type Transaction struct {
	Metadata Metadata

	ID       string
	ParentID string
	TraceID  string

	Timestamp time.Time

	Type           string
	Name           string
	Result         string
	Outcome        string
	Duration       float64
	Marks          TransactionMarks
	Message        *Message
	Sampled        *bool
	SpanCount      SpanCount
	Page           *Page
	HTTP           *Http
	URL            *URL
	Labels         common.MapStr
	Custom         common.MapStr
	UserExperience *UserExperience

	Experimental interface{}

	// RepresentativeCount holds the approximate number of
	// transactions that this transaction represents for aggregation.
	//
	// This may be used for scaling metrics; it is not indexed.
	RepresentativeCount float64
}

type SpanCount struct {
	Dropped *int
	Started *int
}

// fields creates the fields to populate in the top-level "transaction" object field.
func (e *Transaction) fields() common.MapStr {
	var fields mapStr
	fields.set("id", e.ID)
	fields.set("type", e.Type)
	fields.set("duration", utility.MillisAsMicros(e.Duration))
	fields.maybeSetString("name", e.Name)
	fields.maybeSetString("result", e.Result)
	fields.maybeSetMapStr("marks", e.Marks.fields())
	fields.maybeSetMapStr("page", e.Page.Fields())
	fields.maybeSetMapStr("custom", customFields(e.Custom))
	fields.maybeSetMapStr("message", e.Message.Fields())
	fields.maybeSetMapStr("experience", e.UserExperience.Fields())
	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}
		if e.SpanCount.Dropped != nil {
			spanCount["dropped"] = *e.SpanCount.Dropped
		}
		if e.SpanCount.Started != nil {
			spanCount["started"] = *e.SpanCount.Started
		}
		fields.set("span_count", spanCount)
	}
	// TODO(axw) change Sampled to be non-pointer, and set its final value when
	// instantiating the model type.
	fields.set("sampled", e.Sampled == nil || *e.Sampled)
	return common.MapStr(fields)
}

func (e *Transaction) Transform(_ context.Context, cfg *transform.Config) []beat.Event {
	transactionTransformations.Inc()

	fields := common.MapStr{
		"processor":        transactionProcessorEntry,
		transactionDocType: e.fields(),
	}

	if cfg.DataStreams {
		// Transactions are stored in a "traces" data stream along with spans.
		fields[datastreams.TypeField] = datastreams.TracesType
		dataset := fmt.Sprintf("%s.%s", TracesDataset, datastreams.NormalizeServiceName(e.Metadata.Service.Name))
		fields[datastreams.DatasetField] = dataset
	}

	// first set generic metadata (order is relevant)
	e.Metadata.Set(fields, e.Labels)
	utility.Set(fields, "source", fields["client"])

	// then merge event specific information
	utility.AddID(fields, "parent", e.ParentID)
	utility.AddID(fields, "trace", e.TraceID)
	utility.Set(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))
	utility.Set(fields, "http", e.HTTP.Fields())
	urlFields := e.URL.Fields()
	if urlFields != nil {
		utility.Set(fields, "url", e.URL.Fields())
	}
	if e.Page != nil {
		utility.DeepUpdate(fields, "http.request.referrer", e.Page.Referer)
		if urlFields == nil {
			utility.Set(fields, "url", e.Page.URL.Fields())
		}
	}
	utility.DeepUpdate(fields, "event.outcome", e.Outcome)
	utility.Set(fields, "experimental", e.Experimental)
	return []beat.Event{{Fields: fields, Timestamp: e.Timestamp}}
}

type TransactionMarks map[string]TransactionMark

func (m TransactionMarks) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(mapStr, len(m))
	for k, v := range m {
		out.maybeSetMapStr(sanitizeLabelKey(k), v.fields())
	}
	return common.MapStr(out)
}

type TransactionMark map[string]float64

func (m TransactionMark) fields() common.MapStr {
	if len(m) == 0 {
		return nil
	}
	out := make(common.MapStr, len(m))
	for k, v := range m {
		out[sanitizeLabelKey(k)] = common.Float(v)
	}
	return out
}
