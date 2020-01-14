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
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

const (
	spanDocType = "span"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.span", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")

	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")

	processorEntry    = common.MapStr{"name": "transaction", "event": spanDocType}
	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "span")

	errMissingInput = errors.New("input missing for decoding span event")
	errInvalidType  = errors.New("invalid type for span event")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
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

func decodeDB(input interface{}, err error) (*DB, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for db")
	}
	decoder := utility.ManualDecoder{}
	dbInput := decoder.MapStr(raw, "db")
	if decoder.Err != nil || dbInput == nil {
		return nil, decoder.Err
	}
	db := DB{
		decoder.StringPtr(dbInput, "instance"),
		decoder.StringPtr(dbInput, "statement"),
		decoder.StringPtr(dbInput, "type"),
		decoder.StringPtr(dbInput, "user"),
		decoder.StringPtr(dbInput, "link"),
		decoder.IntPtr(dbInput, "rows_affected"),
	}
	return &db, decoder.Err
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

func decodeHTTP(input interface{}, err error) (*HTTP, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for http")
	}
	decoder := utility.ManualDecoder{}
	httpInput := decoder.MapStr(raw, "http")
	if decoder.Err != nil || httpInput == nil {
		return nil, decoder.Err
	}
	method := decoder.StringPtr(httpInput, "method")
	if method != nil {
		*method = strings.ToLower(*method)
	}
	return &HTTP{
		decoder.StringPtr(httpInput, "url"),
		decoder.IntPtr(httpInput, "status_code"),
		method,
	}, decoder.Err
}

func (http *HTTP) fields() common.MapStr {
	if http == nil {
		return nil
	}
	var fields = common.MapStr{}
	if http.URL != nil {
		utility.Set(fields, "url", common.MapStr{"original": http.URL})
	}
	if http.StatusCode != nil {
		utility.Set(fields, "response", common.MapStr{"status_code": http.StatusCode})
	}
	utility.Set(fields, "method", http.Method)
	return fields
}

func decodeDestination(input interface{}, err error) (*Destination, *DestinationService, error) {
	if input == nil || err != nil {
		return nil, nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("invalid type for destination")
	}
	decoder := utility.ManualDecoder{}
	destinationInput := decoder.MapStr(raw, "destination")
	if decoder.Err != nil || destinationInput == nil {
		return nil, nil, decoder.Err
	}
	serviceInput := decoder.MapStr(destinationInput, "service")
	if decoder.Err != nil {
		return nil, nil, decoder.Err
	}
	var service *DestinationService
	if serviceInput != nil {
		service = &DestinationService{
			Type:     decoder.StringPtr(serviceInput, "type"),
			Name:     decoder.StringPtr(serviceInput, "name"),
			Resource: decoder.StringPtr(serviceInput, "resource"),
		}
	}
	dest := Destination{
		Address: decoder.StringPtr(destinationInput, "address"),
		Port:    decoder.IntPtr(destinationInput, "port"),
	}
	return &dest, service, decoder.Err
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

// DecodeEvent decodes a span event.
func DecodeEvent(input interface{}, cfg m.Config, err error) (transform.Transformable, error) {
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, errMissingInput
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errInvalidType
	}

	decoder := utility.ManualDecoder{}
	event := Event{
		Name:          decoder.String(raw, "name"),
		Start:         decoder.Float64Ptr(raw, "start"),
		Duration:      decoder.Float64(raw, "duration"),
		Sync:          decoder.BoolPtr(raw, "sync"),
		Timestamp:     decoder.TimeEpochMicro(raw, "timestamp"),
		Id:            decoder.String(raw, "id"),
		ParentId:      decoder.String(raw, "parent_id"),
		TraceId:       decoder.String(raw, "trace_id"),
		TransactionId: decoder.StringPtr(raw, "transaction_id"),
		Type:          decoder.String(raw, "type"),
		Subtype:       decoder.StringPtr(raw, "subtype"),
		Action:        decoder.StringPtr(raw, "action"),
	}

	ctx := decoder.MapStr(raw, "context")
	if ctx != nil {
		if labels, ok := ctx["tags"].(map[string]interface{}); ok {
			event.Labels = labels
		}

		db, err := decodeDB(ctx, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.DB = db

		http, err := decodeHTTP(ctx, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.HTTP = http

		dest, destService, err := decodeDestination(ctx, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.Destination = dest
		event.DestinationService = destService

		if s, set := ctx["service"]; set {
			service, err := metadata.DecodeService(s, decoder.Err)
			if err != nil {
				return nil, err
			}
			event.Service = service
		}

		if event.Message, err = m.DecodeMessage(ctx, decoder.Err); err != nil {
			return nil, err
		}

		if cfg.Experimental {
			if obj, set := ctx["experimental"]; set {
				event.Experimental = obj
			}
		}
	}

	var stacktr *m.Stacktrace
	stacktr, decoder.Err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	if stacktr != nil {
		event.Stacktrace = *stacktr
	}

	if event.Subtype == nil && event.Action == nil {
		sep := "."
		t := strings.Split(event.Type, sep)
		event.Type = t[0]
		if len(t) > 1 {
			event.Subtype = &t[1]
		}
		if len(t) > 2 {
			action := strings.Join(t[2:], sep)
			event.Action = &action
		}
	}

	return &event, nil
}

func (e *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}

	fields := common.MapStr{
		"processor": processorEntry,
		spanDocType: e.fields(tctx),
	}

	// first set the generic metadata
	tctx.Metadata.SetMinimal(fields)

	// then add event specific information
	utility.DeepUpdate(fields, "service", e.Service.MinimalFields())
	utility.DeepUpdate(fields, "agent", e.Service.AgentFields())
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels)
	utility.AddId(fields, "parent", &e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.AddId(fields, "transaction", e.TransactionId)
	utility.Set(fields, "experimental", e.Experimental)
	utility.Set(fields, "destination", e.Destination.fields())

	timestamp := e.Timestamp
	if timestamp.IsZero() {
		timestamp = tctx.RequestTime
	}

	// adjust timestamp to be reqTime + start
	if e.Timestamp.IsZero() && e.Start != nil {
		timestamp = tctx.RequestTime.Add(time.Duration(float64(time.Millisecond) * *e.Start))
	}

	utility.Set(fields, "timestamp", utility.TimeAsMicros(timestamp))

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: timestamp,
		},
	}
}

func (e *Event) fields(tctx *transform.Context) common.MapStr {
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

	st := e.Stacktrace.Transform(tctx)
	utility.Set(fields, "stacktrace", st)
	return fields
}
