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
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	spanSchema      = validation.CreateSchema(schema.ModelSchema, "span")
	rumV3SpanSchema = validation.CreateSchema(schema.RUMV3Schema, "span")
)

// DecodeRUMV3Span decodes a v3 RUM span.
func DecodeRUMV3Span(input Input) (*span.Event, error) {
	return decodeSpan(input, rumV3SpanSchema)
}

// DecodeSpan decodes a v2 span.
func DecodeSpan(input Input) (transform.Transformable, error) {
	return decodeSpan(input, spanSchema)
}

func decodeSpan(input Input, schema *jsonschema.Schema) (*span.Event, error) {
	raw, err := validation.ValidateObject(input.Raw, schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate span")
	}

	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	decoder := utility.ManualDecoder{}
	event := span.Event{
		Metadata:  input.Metadata,
		Name:      decoder.String(raw, fieldName("name")),
		Start:     decoder.Float64Ptr(raw, fieldName("start")),
		Duration:  decoder.Float64(raw, fieldName("duration")),
		Sync:      decoder.BoolPtr(raw, fieldName("sync")),
		Timestamp: decoder.TimeEpochMicro(raw, fieldName("timestamp")),
		ID:        decoder.String(raw, fieldName("id")),
		ParentID:  decoder.StringPtr(raw, "parent_id"),
		// ParentIdx comes from RUM V3 payloads only, and used to populate ParentID
		ParentIdx:     decoder.IntPtr(raw, fieldName("parent_idx")),
		TraceID:       decoder.StringPtr(raw, "trace_id"),
		TransactionID: decoder.StringPtr(raw, "transaction_id"),
		Type:          decoder.String(raw, fieldName("type")),
		Subtype:       decoder.StringPtr(raw, fieldName("subtype")),
		Action:        decoder.StringPtr(raw, fieldName("action")),
	}

	ctx := decoder.MapStr(raw, fieldName("context"))
	if ctx != nil {
		if labels, ok := ctx[fieldName("tags")].(map[string]interface{}); ok {
			event.Labels = labels
		}

		db, err := decodeDB(ctx, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.DB = db

		http, err := decodeSpanHTTP(ctx, input.Config.HasShortFieldNames, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.HTTP = http

		dest, destService, err := decodeDestination(ctx, input.Config.HasShortFieldNames, decoder.Err)
		if err != nil {
			return nil, err
		}
		event.Destination = dest
		event.DestinationService = destService

		if s := getObject(ctx, "service"); s != nil {
			var service metadata.Service
			decodeService(s, input.Config.HasShortFieldNames, &service)
			event.Service = &service
		}

		if event.Message, err = decodeMessage(ctx, decoder.Err); err != nil {
			return nil, err
		}

		if input.Config.Experimental {
			if obj, set := ctx["experimental"]; set {
				event.Experimental = obj
			}
		}
	}

	var stacktr *m.Stacktrace
	stacktr, decoder.Err = decodeStacktrace(raw[fieldName("stacktrace")], input.Config.HasShortFieldNames, decoder.Err)
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

	if event.Timestamp.IsZero() {
		timestamp := input.RequestTime
		if event.Start != nil {
			// adjust timestamp to be reqTime + start
			timestamp = timestamp.Add(time.Duration(float64(time.Millisecond) * *event.Start))
		}
		event.Timestamp = timestamp
	}

	return &event, nil
}

func decodeDB(input interface{}, err error) (*span.DB, error) {
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
	db := span.DB{
		Instance:     decoder.StringPtr(dbInput, "instance"),
		Statement:    decoder.StringPtr(dbInput, "statement"),
		Type:         decoder.StringPtr(dbInput, "type"),
		UserName:     decoder.StringPtr(dbInput, "user"),
		Link:         decoder.StringPtr(dbInput, "link"),
		RowsAffected: decoder.IntPtr(dbInput, "rows_affected"),
	}
	return &db, decoder.Err
}

func decodeSpanHTTP(input interface{}, hasShortFieldNames bool, err error) (*span.HTTP, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for http")
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)
	httpInput := decoder.MapStr(raw, fieldName("http"))
	if decoder.Err != nil || httpInput == nil {
		return nil, decoder.Err
	}
	method := decoder.StringPtr(httpInput, fieldName("method"))
	if method != nil {
		*method = strings.ToLower(*method)
	}
	minimalResp, err := decodeMinimalHTTPResponse(httpInput, hasShortFieldNames, decoder.Err)
	if err != nil {
		return nil, err
	}
	return &span.HTTP{
		URL:        decoder.StringPtr(httpInput, fieldName("url")),
		StatusCode: decoder.IntPtr(httpInput, fieldName("status_code")),
		Method:     method,
		Response:   minimalResp,
	}, nil
}

func decodeDestination(input interface{}, hasShortFieldNames bool, err error) (*span.Destination, *span.DestinationService, error) {
	if input == nil || err != nil {
		return nil, nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("invalid type for destination")
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decoder := utility.ManualDecoder{}
	destinationInput := decoder.MapStr(raw, fieldName("destination"))
	if decoder.Err != nil || destinationInput == nil {
		return nil, nil, decoder.Err
	}
	serviceInput := decoder.MapStr(destinationInput, fieldName("service"))
	if decoder.Err != nil {
		return nil, nil, decoder.Err
	}
	var service *span.DestinationService
	if serviceInput != nil {
		service = &span.DestinationService{
			Type:     decoder.StringPtr(serviceInput, fieldName("type")),
			Name:     decoder.StringPtr(serviceInput, fieldName("name")),
			Resource: decoder.StringPtr(serviceInput, fieldName("resource")),
		}
	}
	dest := span.Destination{
		Address: decoder.StringPtr(destinationInput, fieldName("address")),
		Port:    decoder.IntPtr(destinationInput, fieldName("port")),
	}
	return &dest, service, decoder.Err
}
