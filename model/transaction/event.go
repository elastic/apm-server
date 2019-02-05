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

package transaction

import (
	"errors"
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": transactionDocType}
)

var (
	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "transaction")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
	Id       string
	ParentId *string
	TraceId  string

	Timestamp time.Time

	Type      string
	Name      *string
	Result    *string
	Duration  float64
	Marks     common.MapStr
	Sampled   *bool
	SpanCount SpanCount
	Context   *m.Context
	User      *metadata.User
	Page      *m.Page
	Http      *m.Http
	Url       *m.Url
	Labels    *m.Labels
	Custom    *m.Custom
}

type SpanCount struct {
	Dropped *int
	Started *int
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
		return nil, errors.New("Invalid type for transaction event")
	}

	ctx, err := m.DecodeContext(raw, nil)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e := Event{
		Id:        decoder.String(raw, "id"),
		Type:      decoder.String(raw, "type"),
		Name:      decoder.StringPtr(raw, "name"),
		Result:    decoder.StringPtr(raw, "result"),
		Duration:  decoder.Float64(raw, "duration"),
		Context:   ctx,
		Labels:    ctx.Labels,
		Page:      ctx.Page,
		Http:      ctx.Http,
		Url:       ctx.Url,
		Custom:    ctx.Custom,
		User:      ctx.User,
		Marks:     decoder.MapStr(raw, "marks"),
		Sampled:   decoder.BoolPtr(raw, "sampled"),
		Timestamp: decoder.TimeEpochMicro(raw, "timestamp"),
		SpanCount: SpanCount{
			Dropped: decoder.IntPtr(raw, "dropped", "span_count"),
			Started: decoder.IntPtr(raw, "started", "span_count")},
		ParentId: decoder.StringPtr(raw, "parent_id"),
		TraceId:  decoder.String(raw, "trace_id"),
	}

	if decoder.Err != nil {
		return nil, decoder.Err
	}

	return &e, nil
}

func (e *Event) fields(tctx *transform.Context) common.MapStr {
	tx := common.MapStr{"id": e.Id}
	utility.Add(tx, "name", e.Name)
	utility.Add(tx, "duration", utility.MillisAsMicros(e.Duration))
	utility.Add(tx, "type", e.Type)
	utility.Add(tx, "result", e.Result)
	utility.Add(tx, "marks", e.Marks)
	utility.Add(tx, "page", e.Page.Fields())
	utility.Add(tx, "custom", e.Custom.Fields())

	if e.Sampled == nil {
		utility.Add(tx, "sampled", true)
	} else {
		utility.Add(tx, "sampled", e.Sampled)
	}

	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}

		if e.SpanCount.Dropped != nil {
			utility.Add(spanCount, "dropped", *e.SpanCount.Dropped)
		}
		if e.SpanCount.Started != nil {
			utility.Add(spanCount, "started", *e.SpanCount.Started)
		}
		utility.Add(tx, "span_count", spanCount)
	}

	return tx
}

func (e *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	var events []beat.Event

	if e.Timestamp.IsZero() {
		e.Timestamp = tctx.RequestTime
	}

	fields := common.MapStr{
		"processor":        processorEntry,
		transactionDocType: e.fields(tctx),
	}
	utility.Add(fields, "user", e.User.Fields())
	utility.AddId(fields, "parent", e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.Add(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))
	utility.Add(fields, "client", e.User.ClientFields())
	utility.Add(fields, "user_agent", e.User.UserAgentFields())
	utility.Add(fields, "labels", e.Labels.Fields())
	utility.Add(fields, "http", e.Http.Fields())
	utility.Add(fields, "url", e.Url.Fields())

	tctx.Metadata.Merge(fields)

	events = append(events, beat.Event{Fields: fields, Timestamp: e.Timestamp})

	return events
}
