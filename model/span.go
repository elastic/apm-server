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

	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	// SpanProcessor is the Processor value that should be assigned to span events.
	SpanProcessor = Processor{Name: "transaction", Event: "span"}
)

type Span struct {
	ID string

	// Name holds the span name: "SELECT FROM table_name", etc.
	Name string

	// Type holds the span type: "external", "db", etc.
	Type string

	// Kind holds the span kind: "CLIENT", "SERVER", "PRODUCER", "CONSUMER" and "INTERNAL".
	Kind string

	// Subtype holds the span subtype: "http", "sql", etc.
	Subtype string

	// Action holds the span action: "query", "execute", etc.
	Action string

	// SelfTime holds the aggregated span durations, for breakdown metrics.
	SelfTime AggregatedDuration

	Message    *Message
	Stacktrace Stacktrace
	Sync       *bool
	Links      []SpanLink

	DB                 *DB
	DestinationService *DestinationService
	Composite          *Composite

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

// DestinationService contains information about the destination service of a span event
type DestinationService struct {
	Type     string // Deprecated
	Name     string // Deprecated
	Resource string

	// ResponseTime holds aggregated span durations for the destination service resource.
	ResponseTime AggregatedDuration
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

func (d *DestinationService) fields() common.MapStr {
	if d == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("type", d.Type)
	fields.maybeSetString("name", d.Name)
	fields.maybeSetString("resource", d.Resource)
	fields.maybeSetMapStr("response_time", d.ResponseTime.fields())
	return common.MapStr(fields)
}

func (c *Composite) fields() common.MapStr {
	if c == nil {
		return nil
	}
	var fields mapStr
	sumDuration := time.Duration(c.Sum * float64(time.Millisecond))
	fields.set("sum", common.MapStr{"us": int(sumDuration.Microseconds())})
	fields.set("count", c.Count)
	fields.set("compression_strategy", c.CompressionStrategy)

	return common.MapStr(fields)
}

func (e *Span) fields() common.MapStr {
	var span mapStr
	span.maybeSetString("name", e.Name)
	span.maybeSetString("type", e.Type)
	span.maybeSetString("id", e.ID)
	span.maybeSetString("kind", e.Kind)
	span.maybeSetString("subtype", e.Subtype)
	span.maybeSetString("action", e.Action)
	span.maybeSetBool("sync", e.Sync)
	span.maybeSetMapStr("db", e.DB.fields())
	span.maybeSetMapStr("message", e.Message.Fields())
	span.maybeSetMapStr("composite", e.Composite.fields())
	if destinationServiceFields := e.DestinationService.fields(); len(destinationServiceFields) > 0 {
		destinationMap, ok := span["destination"].(common.MapStr)
		if !ok {
			destinationMap = make(common.MapStr)
			span.set("destination", destinationMap)
		}
		destinationMap["service"] = destinationServiceFields
	}
	if st := e.Stacktrace.transform(); len(st) > 0 {
		span.set("stacktrace", st)
	}
	span.maybeSetMapStr("self_time", e.SelfTime.fields())
	if n := len(e.Links); n > 0 {
		links := make([]common.MapStr, n)
		for i, link := range e.Links {
			links[i] = link.fields()
		}
		span.set("links", links)
	}
	return common.MapStr(span)
}
