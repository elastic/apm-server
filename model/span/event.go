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
	"errors"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.span", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")

	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")

	spanDocType = "span"

	processorEntry = common.MapStr{"name": "transaction", "event": spanDocType}

	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "span")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
	Id            string
	TransactionId string
	ParentId      string
	TraceId       string

	Timestamp time.Time

	Name       string
	Start      *float64
	Duration   float64
	Context    common.MapStr
	Stacktrace m.Stacktrace
	Sync       *bool
	Labels     common.MapStr

	Type    string
	Subtype *string
	Action  *string

	Db   *db
	Http *http
}
type db struct {
	Instance  *string
	Statement *string
	Type      *string
	UserName  *string
}

func DecodeDb(input interface{}, err error) (*db, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(common.MapStr)
	if !ok {
		return nil, errors.New("Invalid type for db")
	}
	decoder := utility.ManualDecoder{}
	dbInput := decoder.MapStr(raw, "db")
	if decoder.Err != nil || dbInput == nil {
		return nil, decoder.Err
	}
	db := db{
		decoder.StringPtr(dbInput, "instance"),
		decoder.StringPtr(dbInput, "statement"),
		decoder.StringPtr(dbInput, "type"),
		decoder.StringPtr(dbInput, "user"),
	}
	return &db, decoder.Err
}

func (db *db) fields() common.MapStr {
	if db == nil {
		return nil
	}
	var fields = common.MapStr{}
	utility.Add(fields, "instance", db.Instance)
	utility.Add(fields, "statement", db.Statement)
	utility.Add(fields, "type", db.Type)
	if db.UserName != nil {
		utility.Add(fields, "user", common.MapStr{"name": db.UserName})
	}
	return fields
}

type http struct {
	Url        *string
	StatusCode *int
	Method     *string
}

func DecodeHttp(input interface{}, err error) (*http, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(common.MapStr)
	if !ok {
		return nil, errors.New("Invalid type for http")
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
	http := http{
		decoder.StringPtr(httpInput, "url"),
		decoder.IntPtr(httpInput, "status_code"),
		method,
	}
	return &http, decoder.Err
}

func (http *http) fields() common.MapStr {
	if http == nil {
		return nil
	}
	var fields = common.MapStr{}
	if http.Url != nil {
		utility.Add(fields, "url", common.MapStr{"original": http.Url})
	}
	if http.StatusCode != nil {
		utility.Add(fields, "response", common.MapStr{"status_code": http.StatusCode})
	}
	utility.Add(fields, "method", http.Method)
	return fields
}

func DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, errors.New("Input missing for decoding Event")
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for span")
	}

	decoder := utility.ManualDecoder{}
	event := Event{
		Name:          decoder.String(raw, "name"),
		Start:         decoder.Float64Ptr(raw, "start"),
		Duration:      decoder.Float64(raw, "duration"),
		Context:       decoder.MapStr(raw, "context"),
		Sync:          decoder.BoolPtr(raw, "sync"),
		Timestamp:     decoder.TimeEpochMicro(raw, "timestamp"),
		Id:            decoder.String(raw, "id"),
		ParentId:      decoder.String(raw, "parent_id"),
		TraceId:       decoder.String(raw, "trace_id"),
		TransactionId: decoder.String(raw, "transaction_id"),
		Type:          decoder.String(raw, "type"),
		Subtype:       decoder.StringPtr(raw, "subtype"),
		Action:        decoder.StringPtr(raw, "action"),
	}

	if labels, ok := event.Context["tags"].(map[string]interface{}); ok {
		delete(event.Context, "tags")
		event.Labels = labels
	}

	db, err := DecodeDb(event.Context, decoder.Err)
	if err != nil {
		return nil, err
	}
	event.Db = db

	http, err := DecodeHttp(event.Context, nil)
	if err != nil {
		return nil, err
	}
	event.Http = http

	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], nil)
	if stacktr != nil {
		event.Stacktrace = *stacktr
	}

	if decoder.Err != nil {
		return nil, decoder.Err
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
	delete(e.Context, "http")
	delete(e.Context, "db")
	utility.Add(fields, "labels", e.Labels)
	utility.Add(fields, "context", e.Context)
	utility.AddId(fields, "parent", &e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.AddId(fields, "transaction", &e.TransactionId)
	tctx.Metadata.MergeMinimal(fields)

	timestamp := e.Timestamp
	if timestamp.IsZero() {
		timestamp = tctx.RequestTime
	}

	// adjust timestamp to be reqTime + start
	if e.Timestamp.IsZero() && e.Start != nil {
		timestamp = tctx.RequestTime.Add(time.Duration(float64(time.Millisecond) * *e.Start))
	}

	utility.Add(fields, "timestamp", utility.TimeAsMicros(timestamp))

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
	tr := common.MapStr{}
	if e.Id != "" {
		utility.Add(tr, "id", e.Id)
	}
	utility.Add(tr, "subtype", e.Subtype)
	utility.Add(tr, "action", e.Action)

	// common
	utility.Add(tr, "name", e.Name)
	utility.Add(tr, "type", e.Type)
	utility.Add(tr, "sync", e.Sync)

	if e.Start != nil {
		utility.Add(tr, "start", utility.MillisAsMicros(*e.Start))
	}

	utility.Add(tr, "duration", utility.MillisAsMicros(e.Duration))

	utility.Add(tr, "db", e.Db.fields())
	utility.Add(tr, "http", e.Http.fields())

	st := e.Stacktrace.Transform(tctx)
	utility.Add(tr, "stacktrace", st)
	return tr
}
