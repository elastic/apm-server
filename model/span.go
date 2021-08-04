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
	spanDocType = "span"
)

var (
	spanMetrics           = monitoring.Default.NewRegistry("apm-server.processor.span")
	spanTransformations   = monitoring.NewInt(spanMetrics, "transformations")
	spanStacktraceCounter = monitoring.NewInt(spanMetrics, "stacktraces")
	spanFrameCounter      = monitoring.NewInt(spanMetrics, "frames")
	spanProcessorEntry    = common.MapStr{"name": "transaction", "event": spanDocType}
)

type Span struct {
	ID            string
	TransactionID string
	ParentID      string
	ChildIDs      []string
	TraceID       string

	Message    *Message
	Name       string
	Start      *float64
	Duration   float64
	Stacktrace Stacktrace
	Sync       *bool

	Type    string
	Subtype string
	Action  string

	DB                 *DB
	HTTP               *HTTP
	Destination        *Destination
	DestinationService *DestinationService
	Composite          *Composite

	Experimental interface{}

	// RepresentativeCount holds the approximate number of spans that
	// this span represents for aggregation. This will only be set when
	// the sampling rate is known.
	//
	// This may be used for scaling metrics; it is not indexed.
	RepresentativeCount float64
}

// DB contains information related to a database query of a span event
type DB struct {
	Instance     string
	Statement    string
	Type         string
	UserName     string
	Link         string
	RowsAffected *int
}

// Destination contains contextual data about the destination of a span, such as address and port
type Destination struct {
	Address string
	Port    int
}

// DestinationService contains information about the destination service of a span event
type DestinationService struct {
	Type     string // Deprecated
	Name     string // Deprecated
	Resource string
}

// Composite holds details on a group of spans compressed into one.
type Composite struct {
	Count               int
	Sum                 float64 // milliseconds
	CompressionStrategy string
}

func (db *DB) fields() common.MapStr {
	if db == nil {
		return nil
	}
	var fields, user mapStr
	fields.maybeSetString("instance", db.Instance)
	fields.maybeSetString("statement", db.Statement)
	fields.maybeSetString("type", db.Type)
	fields.maybeSetString("link", db.Link)
	fields.maybeSetIntptr("rows_affected", db.RowsAffected)
	if user.maybeSetString("name", db.UserName) {
		fields.set("user", common.MapStr(user))
	}
	return common.MapStr(fields)
}

func (d *Destination) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields mapStr
	if d.Address != "" {
		fields.set("address", d.Address)
	}
	if d.Port > 0 {
		fields.set("port", d.Port)
	}
	return common.MapStr(fields)
}

func (d *DestinationService) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("type", d.Type)
	fields.maybeSetString("name", d.Name)
	fields.maybeSetString("resource", d.Resource)
	return common.MapStr(fields)
}

func (c *Composite) fields() common.MapStr {
	if c == nil {
		return nil
	}
	var fields mapStr
	fields.set("count", c.Count)
	fields.set("sum", utility.MillisAsMicros(c.Sum))
	fields.set("compression_strategy", c.CompressionStrategy)

	return common.MapStr(fields)
}

func (e *Span) fields(apmEvent *APMEvent) common.MapStr {
	spanTransformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		spanStacktraceCounter.Inc()
		spanFrameCounter.Add(int64(frames))
	}

	fields := mapStr{"processor": spanProcessorEntry}

	var trace, transaction, parent mapStr
	if trace.maybeSetString("id", e.TraceID) {
		fields.set("trace", common.MapStr(trace))
	}
	if transaction.maybeSetString("id", e.TransactionID) {
		fields.set("transaction", common.MapStr(transaction))
	}
	if parent.maybeSetString("id", e.ParentID) {
		fields.set("parent", common.MapStr(parent))
	}
	if len(e.ChildIDs) > 0 {
		var child mapStr
		child.set("id", e.ChildIDs)
		fields.set("child", common.MapStr(child))
	}
	if e.Experimental != nil {
		fields.set("experimental", e.Experimental)
	}
	fields.maybeSetMapStr("destination", e.Destination.fields())
	if e.HTTP != nil {
		fields.maybeSetMapStr("http", e.HTTP.spanTopLevelFields())
	}

	var span mapStr
	span.set("name", e.Name)
	span.set("type", e.Type)
	span.maybeSetString("id", e.ID)
	span.maybeSetString("subtype", e.Subtype)
	span.maybeSetString("action", e.Action)
	span.maybeSetBool("sync", e.Sync)
	if e.Start != nil {
		span.set("start", utility.MillisAsMicros(*e.Start))
	}
	span.set("duration", utility.MillisAsMicros(e.Duration))

	if e.HTTP != nil {
		span.maybeSetMapStr("http", e.HTTP.spanFields())
		span.maybeSetString("http.url.original", apmEvent.URL.Original)
	}
	span.maybeSetMapStr("db", e.DB.fields())
	span.maybeSetMapStr("message", e.Message.Fields())
	span.maybeSetMapStr("composite", e.Composite.fields())
	if destinationServiceFields := e.DestinationService.fields(); len(destinationServiceFields) > 0 {
		common.MapStr(span).Put("destination.service", destinationServiceFields)
	}
	// TODO(axw) we should be using a merged service object, combining
	// the stream metadata and event-specific service info.
	if st := e.Stacktrace.transform(); len(st) > 0 {
		span.set("stacktrace", st)
	}
	fields.set("span", common.MapStr(span))

	return common.MapStr(fields)
}
