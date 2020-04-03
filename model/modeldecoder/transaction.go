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
	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/pkg/errors"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	transactionSchema      = validation.CreateSchema(schema.ModelSchema, "transaction")
	rumV3TransactionSchema = validation.CreateSchema(schema.RUMV3Schema, "transaction")
)

func DecodeRUMV3Transaction(input Input) (transform.Transformable, error) {
	event, err := decodeTransaction(input, rumV3TransactionSchema)
	if err != nil {
		return nil, err
	}
	marks, err := decodeRUMV3Marks(input.Raw.(map[string]interface{}), input.Config)
	if err != nil {
		return nil, err
	}
	event.Marks = marks
	return event, nil
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

func DecodeTransaction(input Input) (transform.Transformable, error) {
	return decodeTransaction(input, transactionSchema)
}

func decodeTransaction(input Input, schema *jsonschema.Schema) (*transaction.Event, error) {
	raw, err := validation.ValidateObject(input.Raw, schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate transaction")
	}

	ctx, err := DecodeContext(raw, input.Config, nil)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	e := transaction.Event{
		Metadata:     input.Metadata,
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
		SpanCount: transaction.SpanCount{
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
