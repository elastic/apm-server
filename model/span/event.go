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

package span

import (
	"context"
	"net"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
)

const (
	spanDocType = "span"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.span")
	transformations = monitoring.NewInt(Metrics, "transformations")

	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")

	processorEntry = common.MapStr{"name": "transaction", "event": spanDocType}
)

type Event struct {
	Metadata      metadata.Metadata
	Id            string
	TransactionId *string
	ParentId      string
	TraceId       string

	Timestamp time.Time

	Message    *m.Message
	Name       string
	Start      *float64
	Duration   float64
	Service    *metadata.Service
	Stacktrace m.Stacktrace
	Sync       *bool
	Labels     common.MapStr

	Type    string
	Subtype *string
	Action  *string

	DB                 *DB
	HTTP               *HTTP
	Destination        *Destination
	DestinationService *DestinationService

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
type HTTP struct {
	URL        *string
	StatusCode *int
	Method     *string
	Response   *m.MinimalResp
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

func (e *Event) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}

	fields := common.MapStr{
		"processor": processorEntry,
		spanDocType: e.fields(ctx, tctx),
	}

	// first set the generic metadata
	e.Metadata.Set(fields)

	// then add event specific information
	utility.DeepUpdate(fields, "service", e.Service.Fields("", ""))
	utility.DeepUpdate(fields, "agent", e.Service.AgentFields())
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels)
	utility.AddId(fields, "parent", &e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.AddId(fields, "transaction", e.TransactionId)
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

func (e *Event) fields(ctx context.Context, tctx *transform.Context) common.MapStr {
	if e == nil {
		return nil
	}
	fields := common.MapStr{}
	if e.Id != "" {
		utility.Set(fields, "id", e.Id)
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
	st := e.Stacktrace.Transform(ctx, tctx, &e.Metadata.Service)
	utility.Set(fields, "stacktrace", st)
	return fields
}
