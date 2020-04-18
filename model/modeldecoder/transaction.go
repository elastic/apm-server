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

package modeldecoder

import (
	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	transactionSchema      = validation.CreateSchema(schema.ModelSchema, "transaction")
	rumV3TransactionSchema = validation.CreateSchema(schema.RUMV3Schema, "transaction")
)

// DecodeRUMV3Transaction decodes a v3 RUM transaction.
func DecodeRUMV3Transaction(input Input) (transform.Transformable, error) {
	tr, err := decodeTransaction(input, rumV3TransactionSchema)
	if err != nil {
		return nil, err
	}
	raw := input.Raw.(map[string]interface{})
	spans, err := decodeRUMV3Spans(raw, input, tr)
	if err != nil {
		return nil, err
	}
	event := &transaction.RUMV3Event{
		Event: tr,
		Spans: spans,
	}
	marks, err := decodeRUMV3Marks(raw, input.Config)
	if err != nil {
		return nil, err
	}
	event.Marks = marks
	return event, nil
}

func decodeRUMV3Spans(raw map[string]interface{}, input Input, tr *transaction.Event) ([]span.Event, error) {
	decoder := &utility.ManualDecoder{}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	rawSpans := decoder.InterfaceArr(raw, fieldName("span"))
	var spans = make([]span.Event, len(rawSpans))
	for idx, rawSpan := range rawSpans {
		span, err := DecodeRUMV3Span(Input{
			Raw:         rawSpan,
			RequestTime: input.RequestTime,
			Metadata:    input.Metadata,
			Config:      input.Config,
		})
		if err != nil {
			return spans, err
		}
		span.TransactionID = &tr.ID
		span.TraceID = &tr.TraceID
		if span.ParentIdx == nil {
			span.ParentID = &tr.ID
		} else if *span.ParentIdx < idx {
			span.ParentID = &spans[*span.ParentIdx].ID
		}
		spans[idx] = *span
	}
	return spans, nil
}

// DecodeTransaction decodes a v2 transaction.
func DecodeTransaction(input Input) (transform.Transformable, error) {
	return decodeTransaction(input, transactionSchema)
}

func decodeTransaction(input Input, schema *jsonschema.Schema) (*transaction.Event, error) {
	raw, err := validation.ValidateObject(input.Raw, schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate transaction")
	}

	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	ctx, err := decodeContext(getObject(raw, fieldName("context")), input.Config, &input.Metadata)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e := transaction.Event{
		Metadata:     input.Metadata,
		ID:           decoder.String(raw, "id"),
		Type:         decoder.String(raw, fieldName("type")),
		Name:         decoder.StringPtr(raw, fieldName("name")),
		Result:       decoder.StringPtr(raw, fieldName("result")),
		Duration:     decoder.Float64(raw, fieldName("duration")),
		Labels:       ctx.Labels,
		Page:         ctx.Page,
		Http:         ctx.Http,
		Url:          ctx.Url,
		Custom:       ctx.Custom,
		Client:       ctx.Client,
		Experimental: ctx.Experimental,
		Message:      ctx.Message,
		Sampled:      decoder.BoolPtr(raw, fieldName("sampled")),
		Marks:        decoder.MapStr(raw, fieldName("marks")),
		Timestamp:    decoder.TimeEpochMicro(raw, fieldName("timestamp")),
		SpanCount: transaction.SpanCount{
			Dropped: decoder.IntPtr(raw, fieldName("dropped"), fieldName("span_count")),
			Started: decoder.IntPtr(raw, fieldName("started"), fieldName("span_count"))},
		ParentID: decoder.StringPtr(raw, fieldName("parent_id")),
		TraceID:  decoder.String(raw, fieldName("trace_id")),
	}
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = input.RequestTime
	}
	return &e, nil
}

func decodeRUMV3Marks(raw map[string]interface{}, cfg Config) (common.MapStr, error) {

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

	if err := decoder.Err; err != nil {
		return nil, err
	}
	return common.MapStr{
		"agent":            agentMarks,
		"navigationTiming": navigationTiming,
	}, nil
}
