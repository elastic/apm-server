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
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

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
	ID       string
	ParentID string

	Type           string
	Name           string
	Result         string
	Duration       float64
	Marks          TransactionMarks
	Message        *Message
	Sampled        bool
	SpanCount      SpanCount
	HTTP           *HTTP
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

func (e *Transaction) fields() common.MapStr {
	transactionTransformations.Inc()

	fields := mapStr{
		"processor": transactionProcessorEntry,
	}

	var parent mapStr
	parent.maybeSetString("id", e.ParentID)
	fields.maybeSetMapStr("parent", common.MapStr(parent))
	if e.HTTP != nil {
		fields.maybeSetMapStr("http", e.HTTP.transactionTopLevelFields())
	}
	if e.Experimental != nil {
		fields.set("experimental", e.Experimental)
	}

	var transaction mapStr
	transaction.set("id", e.ID)
	transaction.set("type", e.Type)
	transaction.set("duration", utility.MillisAsMicros(e.Duration))
	transaction.maybeSetString("name", e.Name)
	transaction.maybeSetString("result", e.Result)
	transaction.maybeSetMapStr("marks", e.Marks.fields())
	transaction.maybeSetMapStr("custom", customFields(e.Custom))
	transaction.maybeSetMapStr("message", e.Message.Fields())
	transaction.maybeSetMapStr("experience", e.UserExperience.Fields())
	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}
		if e.SpanCount.Dropped != nil {
			spanCount["dropped"] = *e.SpanCount.Dropped
		}
		if e.SpanCount.Started != nil {
			spanCount["started"] = *e.SpanCount.Started
		}
		transaction.set("span_count", spanCount)
	}
	transaction.set("sampled", e.Sampled)
	fields.set("transaction", common.MapStr(transaction))

	return common.MapStr(fields)
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
