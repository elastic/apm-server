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
	"net"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

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
	Start      *float64
	Duration   float64
	Service    *Service
	Stacktrace Stacktrace
	Sync       *bool
	Labels     common.MapStr

	Type    string
	Subtype *string
	Action  *string

	DB                 *DB
	HTTP               *HTTP
	Destination        *Destination
	DestinationService *DestinationService

	// RUM records whether or not this is a RUM span,
	// and should have its stack frames sourcemapped.
	RUM bool

	Experimental interface{}
}

// DB contains information related to a database query of a span event
type DB struct {
	Instance     *string
	Statement    *string
	Type         *string
	UserName     *string
	Link         *string
	RowsAffected *int
}

// HTTP contains information about the outgoing http request information of a span event
//
// TODO(axw) combine this and "Http", which is used by transaction and error, into one type.
type HTTP struct {
	URL        *string
	StatusCode *int
	Method     *string
	Response   *MinimalResp
}

// Destination contains contextual data about the destination of a span, such as address and port
type Destination struct {
	Address *string
	Port    *int
}

// DestinationService contains information about the destination service of a span event
type DestinationService struct {
	Type     *string
	Name     *string
	Resource *string
}

func (db *DB) fields() common.MapStr {
	if db == nil {
		return nil
	}
	var fields = common.MapStr{}
	utility.Set(fields, "instance", db.Instance)
	utility.Set(fields, "statement", db.Statement)
	utility.Set(fields, "type", db.Type)
	utility.Set(fields, "rows_affected", db.RowsAffected)
	if db.UserName != nil {
		utility.Set(fields, "user", common.MapStr{"name": db.UserName})
	}
	utility.Set(fields, "link", db.Link)
	return fields
}

func (http *HTTP) fields() common.MapStr {
	if http == nil {
		return nil
	}
	var fields = common.MapStr{}
	if http.URL != nil {
		utility.Set(fields, "url", common.MapStr{"original": http.URL})
	}
	response := http.Response.Fields()
	if http.StatusCode != nil {
		if response == nil {
			response = common.MapStr{"status_code": *http.StatusCode}
		} else if http.Response.StatusCode == nil {
			response["status_code"] = *http.StatusCode
		}
	}
	utility.Set(fields, "response", response)
	utility.Set(fields, "method", http.Method)
	return fields
}

func (d *Destination) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields = common.MapStr{}
	if d.Address != nil {
		address := *d.Address
		fields["address"] = address
		if ip := net.ParseIP(address); ip != nil {
			fields["ip"] = address
		}
	}
	utility.Set(fields, "port", d.Port)
	return fields
}

func (d *DestinationService) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields = common.MapStr{}
	utility.Set(fields, "type", d.Type)
	utility.Set(fields, "name", d.Name)
	utility.Set(fields, "resource", d.Resource)
	return fields
}

func (e *Span) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	spanTransformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		spanStacktraceCounter.Inc()
		spanFrameCounter.Add(int64(frames))
	}

	fields := common.MapStr{
		"processor": spanProcessorEntry,
		spanDocType: e.fields(ctx, tctx),
	}

	// first set the generic metadata
	e.Metadata.Set(fields)

	// then add event specific information
	utility.DeepUpdate(fields, "service", e.Service.Fields("", ""))
	utility.DeepUpdate(fields, "agent", e.Service.AgentFields())
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels)
	utility.AddID(fields, "parent", e.ParentID)
	if e.ChildIDs != nil {
		utility.Set(fields, "child", common.MapStr{"id": e.ChildIDs})
	}
	utility.AddID(fields, "trace", e.TraceID)
	utility.AddID(fields, "transaction", e.TransactionID)
	utility.Set(fields, "experimental", e.Experimental)
	utility.Set(fields, "destination", e.Destination.fields())
	utility.Set(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: e.Timestamp,
		},
	}
}

func (e *Span) fields(ctx context.Context, tctx *transform.Context) common.MapStr {
	if e == nil {
		return nil
	}
	fields := common.MapStr{}
	if e.ID != "" {
		utility.Set(fields, "id", e.ID)
	}
	utility.Set(fields, "subtype", e.Subtype)
	utility.Set(fields, "action", e.Action)

	// common
	utility.Set(fields, "name", e.Name)
	utility.Set(fields, "type", e.Type)
	utility.Set(fields, "sync", e.Sync)

	if e.Start != nil {
		utility.Set(fields, "start", utility.MillisAsMicros(*e.Start))
	}

	utility.Set(fields, "duration", utility.MillisAsMicros(e.Duration))

	utility.Set(fields, "db", e.DB.fields())
	utility.Set(fields, "http", e.HTTP.fields())
	utility.DeepUpdate(fields, "destination.service", e.DestinationService.fields())

	utility.Set(fields, "message", e.Message.Fields())

	// TODO(axw) we should be using a merged service object, combining
	// the stream metadata and event-specific service info.
	st := e.Stacktrace.transform(ctx, tctx, &e.Metadata.Service)
	utility.Set(fields, "stacktrace", st)
	return fields
}
