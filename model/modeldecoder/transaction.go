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
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/field"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

var (
	transactionSchema      = validation.CreateSchema(schema.ModelSchema, "transaction")
	rumV3TransactionSchema = validation.CreateSchema(schema.RUMV3Schema, "transaction")
)

// DecodeRUMV3Transaction decodes a v3 RUM transaction.
func DecodeRUMV3Transaction(input Input, batch *model.Batch) error {
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	transaction, err := decodeTransaction(input, rumV3TransactionSchema)
	if err != nil {
		return err
	}
	raw := input.Raw.(map[string]interface{})
	spans, err := decodeRUMV3Spans(raw, input, transaction)
	if err != nil {
		return err
	}
	transaction.Marks = decodeRUMV3Marks(getObject(raw, fieldName("marks")), input.Config)
	metricsets, err := decodeRUMV3Metricsets(raw, input, transaction)
	if err != nil {
		return nil
	}
	batch.Transactions = append(batch.Transactions, transaction)
	batch.Spans = append(batch.Spans, spans...)
	batch.Metricsets = append(batch.Metricsets, metricsets...)
	return nil
}

func decodeRUMV3Metricsets(raw map[string]interface{}, input Input, tr *model.Transaction) ([]*model.Metricset, error) {
	decoder := &utility.ManualDecoder{}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	rawMetricsets := decoder.InterfaceArr(raw, fieldName("metricset"))
	var metricsets = make([]*model.Metricset, len(rawMetricsets))
	for idx, rawMetricset := range rawMetricsets {
		metricset, err := decodeMetricset(Input{
			Raw:         rawMetricset,
			RequestTime: input.RequestTime,
			Metadata:    input.Metadata,
			Config:      input.Config,
		}, rumV3Schema)
		if err != nil {
			return metricsets, err
		}
		metricset.Transaction = model.MetricsetTransaction{
			Type:   tr.Type,
			Name:   tr.Name,
			Result: tr.Result,
		}
		metricsets[idx] = metricset
	}
	return metricsets, nil
}

func decodeRUMV3Spans(raw map[string]interface{}, input Input, tr *model.Transaction) ([]*model.Span, error) {
	decoder := &utility.ManualDecoder{}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)
	rawSpans := decoder.InterfaceArr(raw, fieldName("span"))
	var spans = make([]*model.Span, len(rawSpans))
	for idx, rawSpan := range rawSpans {
		span, parentIndex, err := decodeRUMV3Span(Input{
			Raw:         rawSpan,
			RequestTime: input.RequestTime,
			Metadata:    input.Metadata,
			Config:      input.Config,
		})
		if err != nil {
			return spans, err
		}
		span.TransactionID = tr.ID
		span.TraceID = tr.TraceID
		if parentIndex >= 0 && parentIndex < idx {
			span.ParentID = spans[parentIndex].ID
		} else {
			span.ParentID = tr.ID
		}
		spans[idx] = span
	}
	return spans, nil
}

// DecodeRUMV2Transaction decodes a v2 RUM transaction.
func DecodeRUMV2Transaction(input Input, batch *model.Batch) error {
	// Identical to backend agent transactions.
	return DecodeTransaction(input, batch)
}

// DecodeTransaction decodes a v2 transaction.
func DecodeTransaction(input Input, batch *model.Batch) error {
	transaction, err := decodeTransaction(input, transactionSchema)
	if err != nil {
		return err
	}
	batch.Transactions = append(batch.Transactions, transaction)
	return nil
}

func decodeTransaction(input Input, schema *jsonschema.Schema) (*model.Transaction, error) {
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
	e := model.Transaction{
		Metadata:            input.Metadata,
		Labels:              ctx.Labels,
		Page:                ctx.Page,
		HTTP:                ctx.Http,
		URL:                 ctx.URL,
		Custom:              ctx.Custom,
		Experimental:        ctx.Experimental,
		Message:             ctx.Message,
		Sampled:             decoder.BoolPtr(raw, fieldName("sampled")),
		RepresentativeCount: safeInverse(decoder.Float64WithDefault(raw, 1.0, fieldName("sample_rate"))),
		Marks:               decodeV2Marks(getObject(raw, fieldName("marks"))),
		Timestamp:           decoder.TimeEpochMicro(raw, fieldName("timestamp")),
		SpanCount: model.SpanCount{
			Dropped: decoder.IntPtr(raw, fieldName("dropped"), fieldName("span_count")),
			Started: decoder.IntPtr(raw, fieldName("started"), fieldName("span_count"))},
	}
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	decodeString(raw, "id", &e.ID)
	decodeString(raw, fieldName("trace_id"), &e.TraceID)
	decodeString(raw, fieldName("parent_id"), &e.ParentID)
	decodeString(raw, fieldName("type"), &e.Type)
	decodeString(raw, fieldName("name"), &e.Name)
	decodeString(raw, fieldName("result"), &e.Result)
	decodeFloat64(raw, fieldName("duration"), &e.Duration)

	if obj := getObject(raw, fieldName("experience")); obj != nil {
		var experience model.UserExperience
		decodeUserExperience(obj, &experience)
		e.UserExperience = &experience
	}

	if e.Timestamp.IsZero() {
		e.Timestamp = input.RequestTime
	}
	return &e, nil
}

func decodeV2Marks(raw map[string]interface{}) model.TransactionMarks {
	if len(raw) == 0 {
		return nil
	}
	marks := make(model.TransactionMarks, len(raw))
	for group, v := range raw {
		groupObj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		groupMarks := make(model.TransactionMark, len(groupObj))
		for k, v := range groupObj {
			switch v := v.(type) {
			case json.Number:
				if f, err := v.Float64(); err == nil {
					groupMarks[k] = f
				}
			case float64:
				groupMarks[k] = v
			}
		}
		marks[group] = groupMarks
	}
	return marks
}

func decodeRUMV3Marks(raw map[string]interface{}, cfg Config) model.TransactionMarks {
	fieldName := field.Mapper(cfg.HasShortFieldNames)
	marks := make(model.TransactionMarks)
	decodeMarks := func(group string, names ...string) {
		groupObj := getObject(raw, fieldName(group))
		if groupObj == nil {
			return
		}
		groupMarks := make(model.TransactionMark, len(groupObj))
		for _, name := range names {
			var v float64
			if decodeFloat64(groupObj, fieldName(name), &v) {
				groupMarks[name] = v
			}
		}
		if len(groupMarks) != 0 {
			marks[group] = groupMarks
		}
	}

	decodeMarks("agent",
		"domComplete",
		"domInteractive",
		"domContentLoadedEventStart",
		"domContentLoadedEventEnd",
		"timeToFirstByte",
		"firstContentfulPaint",
		"largestContentfulPaint",
	)
	decodeMarks("navigationTiming",
		"fetchStart",
		"domainLookupStart",
		"domainLookupEnd",
		"connectStart",
		"connectEnd",
		"requestStart",
		"responseStart",
		"responseEnd",
		"domComplete",
		"domInteractive",
		"domLoading",
		"domContentLoadedEventStart",
		"domContentLoadedEventEnd",
		"loadEventStart",
		"loadEventEnd",
	)
	return marks
}
