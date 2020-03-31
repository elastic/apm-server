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
	"context"
	"time"

	"github.com/elastic/apm-server/model/field"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/monitoring"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
	emptyString        = ""
)

var (
	Metrics           = monitoring.Default.NewRegistry("apm-server.processor.transaction")
	transformations   = monitoring.NewInt(Metrics, "transformations")
	processorEntry    = common.MapStr{"name": processorName, "event": transactionDocType}
	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "transaction")
	RUMV3Schema       = validation.CreateSchema(schema.RUMV3Schema, "transaction")

	errMissingInput = errors.New("input missing for decoding transaction event")
	errInvalidType  = errors.New("invalid type for transaction event")
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
	Message   *m.Message
	Sampled   *bool
	SpanCount SpanCount
	User      *metadata.User
	Page      *m.Page
	Http      *m.Http
	Url       *m.Url
	Labels    *m.Labels
	Custom    *m.Custom
	Service   *metadata.Service
	Client    *m.Client

	Experimental interface{}
}

type SpanCount struct {
	Dropped *int
	Started *int
}

func DecodeRUMV3Event(input m.Input) (transform.Transformable, error) {
	transformable, err := DecodeEvent(input)
	if err != nil {
		return transformable, err
	}
	event, _ := transformable.(*Event)
	raw, _ := input.Raw.(map[string]interface{})
	return decodeRUMV3Marks(event, raw, input.Config)
}

func decodeRUMV3Marks(event *Event, raw map[string]interface{}, cfg m.Config) (transform.Transformable, error) {

	decoder := &utility.ManualDecoder{}
	fieldName := field.Mapper(cfg.HasShortFieldNames)

	decodeMark := func(m common.MapStr, key, parent string) {
		if f := decoder.Float64Ptr(raw, fieldName(key), fieldName("marks"), fieldName(parent)); f != nil {
			m[key] = f
		}
	}

	agentMarks := common.MapStr{}
	decodeMark(agentMarks, "domComplete", "agent")
	decodeMark(agentMarks, "domInteractive", "agent")
	decodeMark(agentMarks, "domContentLoadedEventStart", "agent")
	decodeMark(agentMarks, "domContentLoadedEventEnd", "agent")
	decodeMark(agentMarks, "timeToFirstByte", "agent")
	decodeMark(agentMarks, "firstContentfulPaint", "agent")
	decodeMark(agentMarks, "largestContentfulPaint", "agent")

	navigationTiming := common.MapStr{}
	decodeMark(navigationTiming, "fetchStart", "navigationTiming")
	decodeMark(navigationTiming, "domainLookupStart", "navigationTiming")
	decodeMark(navigationTiming, "domainLookupEnd", "navigationTiming")
	decodeMark(navigationTiming, "connectStart", "navigationTiming")
	decodeMark(navigationTiming, "connectEnd", "navigationTiming")
	decodeMark(navigationTiming, "requestStart", "navigationTiming")
	decodeMark(navigationTiming, "responseStart", "navigationTiming")
	decodeMark(navigationTiming, "responseEnd", "navigationTiming")
	decodeMark(navigationTiming, "domComplete", "navigationTiming")
	decodeMark(navigationTiming, "domInteractive", "navigationTiming")
	decodeMark(navigationTiming, "domLoading", "navigationTiming")
	decodeMark(navigationTiming, "domContentLoadedEventStart", "navigationTiming")
	decodeMark(navigationTiming, "domContentLoadedEventEnd", "navigationTiming")
	decodeMark(navigationTiming, "loadEventStart", "navigationTiming")
	decodeMark(navigationTiming, "loadEventEnd", "navigationTiming")

	event.Marks = common.MapStr{
		"agent":            agentMarks,
		"navigationTiming": navigationTiming,
	}
	return event, decoder.Err
}

func DecodeEvent(input m.Input) (transform.Transformable, error) {
	if input.Raw == nil {
		return nil, errMissingInput
	}
	raw, ok := input.Raw.(map[string]interface{})
	if !ok {
		return nil, errInvalidType
	}

	ctx, err := m.DecodeContext(raw, input.Config, nil)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	e := Event{
		Id:           decoder.String(raw, "id"),
		Type:         decoder.String(raw, fieldName("type")),
		Name:         decoder.StringPtr(raw, fieldName("name")),
		Result:       decoder.StringPtr(raw, fieldName("result")),
		Duration:     decoder.Float64(raw, fieldName("duration")),
		Labels:       ctx.Labels,
		Page:         ctx.Page,
		Http:         ctx.Http,
		Url:          ctx.Url,
		Custom:       ctx.Custom,
		User:         ctx.User,
		Service:      ctx.Service,
		Client:       ctx.Client,
		Experimental: ctx.Experimental,
		Message:      ctx.Message,
		Sampled:      decoder.BoolPtr(raw, fieldName("sampled")),
		Marks:        decoder.MapStr(raw, fieldName("marks")),
		Timestamp:    decoder.TimeEpochMicro(raw, fieldName("timestamp")),
		SpanCount: SpanCount{
			Dropped: decoder.IntPtr(raw, fieldName("dropped"), fieldName("span_count")),
			Started: decoder.IntPtr(raw, fieldName("started"), fieldName("span_count"))},
		ParentId: decoder.StringPtr(raw, "parent_id"),
		TraceId:  decoder.String(raw, "trace_id"),
	}
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = input.RequestTime
	}

	return &e, nil
}

func (e *Event) fields(tctx *transform.Context) common.MapStr {
	tx := common.MapStr{"id": e.Id}
	utility.Set(tx, "name", e.Name)
	utility.Set(tx, "duration", utility.MillisAsMicros(e.Duration))
	utility.Set(tx, "type", e.Type)
	utility.Set(tx, "result", e.Result)
	utility.Set(tx, "marks", e.Marks)
	utility.Set(tx, "page", e.Page.Fields())
	utility.Set(tx, "custom", e.Custom.Fields())
	utility.Set(tx, "message", e.Message.Fields())

	if e.Sampled == nil {
		utility.Set(tx, "sampled", true)
	} else {
		utility.Set(tx, "sampled", e.Sampled)
	}

	if e.SpanCount.Dropped != nil || e.SpanCount.Started != nil {
		spanCount := common.MapStr{}

		if e.SpanCount.Dropped != nil {
			utility.Set(spanCount, "dropped", *e.SpanCount.Dropped)
		}
		if e.SpanCount.Started != nil {
			utility.Set(spanCount, "started", *e.SpanCount.Started)
		}
		utility.Set(tx, "span_count", spanCount)
	}

	return tx
}

func (e *Event) Transform(ctx context.Context, tctx *transform.Context) []beat.Event {
	transformations.Inc()

	fields := common.MapStr{
		"processor":        processorEntry,
		transactionDocType: e.fields(tctx),
	}

	// first set generic metadata (order is relevant)
	tctx.Metadata.Set(fields)

	// then merge event specific information
	utility.Update(fields, "user", e.User.Fields())
	clientFields := e.Client.Fields()
	utility.DeepUpdate(fields, "client", clientFields)
	utility.DeepUpdate(fields, "source", clientFields)
	utility.DeepUpdate(fields, "user_agent", e.User.UserAgentFields())
	utility.DeepUpdate(fields, "service", e.Service.Fields(emptyString, emptyString))
	utility.DeepUpdate(fields, "agent", e.Service.AgentFields())
	utility.AddId(fields, "parent", e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.Set(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))
	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", e.Labels.Fields())
	utility.Set(fields, "http", e.Http.Fields())
	utility.Set(fields, "url", e.Url.Fields())
	utility.Set(fields, "experimental", e.Experimental)

	return []beat.Event{{Fields: fields, Timestamp: e.Timestamp}}
}
