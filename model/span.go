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
	"net"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	"github.com/elastic/apm-server/datastreams"
	"github.com/elastic/apm-server/transform"
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
	Metadata      Metadata
	ID            string
	TransactionID string
	ParentID      string
	ChildIDs      []string
	TraceID       string

	Timestamp time.Time

	Message    *Message
	Name       string
	Outcome    string
	Start      *float64
	Duration   float64
	Stacktrace Stacktrace
	Sync       *bool
	Labels     common.MapStr

	Type    string
	Subtype string
	Action  string

	DB                 *DB
	HTTP               *HTTP
	Destination        *Destination
	DestinationService *DestinationService

	// RUM records whether or not this is a RUM span,
	// and should have its stack frames sourcemapped.
	RUM bool

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

// HTTP contains information about the outgoing http request information of a span event
//
// TODO(axw) combine this and "Http", which is used by transaction and error, into one type.
type HTTP struct {
	URL        string
	StatusCode int
	Method     string
	Response   *MinimalResp
}

// Destination contains contextual data about the destination of a span, such as address and port
type Destination struct {
	Address string
	Port    int
}

// DestinationService contains information about the destination service of a span event
type DestinationService struct {
	Type     string  // Deprecated
	Name     string  // Deprecated
	Resource string
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

func (http *HTTP) fields() common.MapStr {
	if http == nil {
		return nil
	}
	var fields, url mapStr
	if url.maybeSetString("original", http.URL) {
		fields.set("url", common.MapStr(url))
	}
	response := http.Response.Fields()
	if http.StatusCode > 0 {
		if response == nil {
			response = common.MapStr{"status_code": http.StatusCode}
		} else if http.Response.StatusCode == 0 {
			response["status_code"] = http.StatusCode
		}
	}
	fields.maybeSetMapStr("response", response)
	fields.maybeSetString("method", http.Method)
	return common.MapStr(fields)
}

func (d *Destination) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields mapStr
	if d.Address != "" {
		fields.set("address", d.Address)
		if ip := net.ParseIP(d.Address); ip != nil {
			fields.set("ip", d.Address)
		}
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

func (e *Span) appendBeatEvents(ctx context.Context, cfg *transform.Config, events []beat.Event) []beat.Event {
	spanTransformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		spanStacktraceCounter.Inc()
		spanFrameCounter.Add(int64(frames))
	}

	fields := mapStr{
		"processor": spanProcessorEntry,
		spanDocType: e.fields(ctx, cfg),
	}

	if cfg.DataStreams {
		// Spans are stored in a "traces" data stream along with transactions.
		fields[datastreams.TypeField] = datastreams.TracesType
		dataset := fmt.Sprintf("%s.%s", TracesDataset, datastreams.NormalizeServiceName(e.Metadata.Service.Name))
		fields[datastreams.DatasetField] = dataset
	}

	// first set the generic metadata
	e.Metadata.set(&fields, e.Labels)

	// then add event specific information
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
	fields.maybeSetMapStr("timestamp", utility.TimeAsMicros(e.Timestamp))
	if e.Experimental != nil {
		fields.set("experimental", e.Experimental)
	}
	fields.maybeSetMapStr("destination", e.Destination.fields())

	common.MapStr(fields).Put("event.outcome", e.Outcome)

	return append(events, beat.Event{
		Fields:    common.MapStr(fields),
		Timestamp: e.Timestamp,
	})
}

func (e *Span) fields(ctx context.Context, cfg *transform.Config) common.MapStr {
	if e == nil {
		return nil
	}
	var fields mapStr
	fields.set("name", e.Name)
	fields.set("type", e.Type)
	fields.maybeSetString("id", e.ID)
	fields.maybeSetString("subtype", e.Subtype)
	fields.maybeSetString("action", e.Action)
	fields.maybeSetBool("sync", e.Sync)
	if e.Start != nil {
		fields.set("start", utility.MillisAsMicros(*e.Start))
	}
	fields.set("duration", utility.MillisAsMicros(e.Duration))

	fields.maybeSetMapStr("db", e.DB.fields())
	fields.maybeSetMapStr("http", e.HTTP.fields())
	fields.maybeSetMapStr("message", e.Message.Fields())
	if destinationServiceFields := e.DestinationService.fields(); len(destinationServiceFields) > 0 {
		common.MapStr(fields).Put("destination.service", destinationServiceFields)
	}

	// TODO(axw) we should be using a merged service object, combining
	// the stream metadata and event-specific service info.
	if st := e.Stacktrace.transform(ctx, cfg, e.RUM, &e.Metadata.Service); len(st) > 0 {
		fields.set("stacktrace", st)
	}
	return common.MapStr(fields)
}
