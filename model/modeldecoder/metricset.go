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
	"context"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metricset/generated/schema"
	"github.com/elastic/apm-server/model/modeldecoder/field"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

const (
	transactionKey       = "transaction"
	spanKey              = "span"
	metricTransactionKey = "metricsetTransaction"
)

var (
	metricsetSchema = validation.CreateSchema(schema.ModelSchema, "metricset")
	rumV3Schema     = validation.CreateSchema(schema.RUMV3Schema, "metricset")
)

// DecodeMetricset decodes a v2 metricset.
func DecodeMetricset(ctx context.Context, input Input) (context.Context, transform.Transformable, error) {
	return decodeMetricset(ctx, input, metricsetSchema)
}

func DecodeRUMV3Metricset(ctx context.Context, input Input) (context.Context, transform.Transformable, error) {
	ctx, metricset, err := decodeMetricset(ctx, input, rumV3Schema)
	if err != nil {
		return ctx, nil, err
	}
	if transaction := ctx.Value(metricTransactionKey); transaction != nil {
		metricset.Transaction = transaction.(model.MetricsetTransaction)
	}
	return ctx, metricset, nil
}

func decodeMetricset(ctx context.Context, input Input, schema *jsonschema.Schema) (context.Context, *model.Metricset, error) {
	raw, err := validation.ValidateObject(input.Raw, schema)
	if err != nil {
		return ctx, nil, errors.Wrap(err, "failed to validate metricset")
	}

	md := metricsetDecoder{&utility.ManualDecoder{}}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)

	e := model.Metricset{
		Timestamp: md.TimeEpochMicro(raw, "timestamp"),
		Metadata:  input.Metadata,
	}
	md.decodeSamples(getObject(raw, fieldName("samples")), input.Config.HasShortFieldNames, &e.Samples)
	md.decodeTransaction(getObject(raw, fieldName(transactionKey)), input.Config.HasShortFieldNames, &e.Transaction)
	md.decodeSpan(getObject(raw, fieldName(spanKey)), input.Config.HasShortFieldNames, &e.Span)

	if md.Err != nil {
		return ctx, nil, md.Err
	}

	if tags := utility.Prune(md.MapStr(raw, "tags")); len(tags) > 0 {
		e.Labels = tags
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = input.RequestTime
	}

	return ctx, &e, nil
}

type metricsetDecoder struct {
	*utility.ManualDecoder
}

func (md *metricsetDecoder) decodeSamples(input map[string]interface{}, hasShortFieldNames bool, out *[]model.Sample) {
	fieldName := field.Mapper(hasShortFieldNames)
	inverseFieldName := field.InverseMapper(hasShortFieldNames)

	valueFieldName := fieldName("value")
	for name, s := range input {
		sampleObj, _ := s.(map[string]interface{})
		sample := model.Sample{Name: inverseFieldName(name)}
		// TODO(axw) add support for ingesting counts/values (histogram metrics)
		decodeFloat64(sampleObj, valueFieldName, &sample.Value)
		*out = append(*out, sample)
	}
}

func (md *metricsetDecoder) decodeSpan(input map[string]interface{}, hasShortFieldNames bool, out *model.MetricsetSpan) {
	fieldName := field.Mapper(hasShortFieldNames)
	decodeString(input, fieldName("type"), &out.Type)
	decodeString(input, fieldName("subtype"), &out.Subtype)
}

func (md *metricsetDecoder) decodeTransaction(input map[string]interface{}, hasShortFieldNames bool, out *model.MetricsetTransaction) {
	fieldName := field.Mapper(hasShortFieldNames)
	decodeString(input, fieldName("type"), &out.Type)
	decodeString(input, fieldName("name"), &out.Name)
	// TODO(axw) add support for ingesting transaction.result, transaction.root
}
