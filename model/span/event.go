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
		Type:          decoder.String(raw, "type"),
		Start:         decoder.Float64Ptr(raw, "start"),
		Duration:      decoder.Float64(raw, "duration"),
		Context:       decoder.MapStr(raw, "context"),
		Sync:          decoder.BoolPtr(raw, "sync"),
		Timestamp:     decoder.TimeEpochMicro(raw, "timestamp"),
		Id:            decoder.String(raw, "id"),
		ParentId:      decoder.String(raw, "parent_id"),
		TraceId:       decoder.String(raw, "trace_id"),
		TransactionId: decoder.String(raw, "transaction_id"),
		Subtype:       decoder.StringPtr(raw, "subtype"),
		Action:        decoder.StringPtr(raw, "action"),
	}

	if labels, ok := event.Context["tags"].(map[string]interface{}); ok {
		delete(event.Context, "tags")
		event.Labels = labels
	}

	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		event.Stacktrace = *stacktr
	}

	if decoder.Err != nil {
		return nil, decoder.Err
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

func (s *Event) fields(tctx *transform.Context) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	if s.Id != "" {
		utility.Add(tr, "id", s.Id)
	}
	utility.Add(tr, "subtype", s.Subtype)
	utility.Add(tr, "action", s.Action)

	// common
	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "sync", s.Sync)

	if s.Start != nil {
		utility.Add(tr, "start", utility.MillisAsMicros(*s.Start))
	}

	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))

	st := s.Stacktrace.Transform(tctx)
	utility.Add(tr, "stacktrace", st)
	return tr
}
