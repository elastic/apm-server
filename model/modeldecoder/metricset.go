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
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metricset"
	"github.com/elastic/apm-server/model/metricset/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
)

const (
	transactionKey = "transaction"
	spanKey        = "span"
)

var (
	metricsetSchema = validation.CreateSchema(schema.ModelSchema, "metricset")
	rumV3Schema     = validation.CreateSchema(schema.RUMV3Schema, "metricset")
)

// DecodeMetricset decodes a v2 metricset.
func DecodeMetricset(input Input) (transform.Transformable, error) {
	return decodeMetricset(input, metricsetSchema)
}

func DecodeRUMV3Metricset(input Input) (transform.Transformable, error) {
	return decodeMetricset(input, rumV3Schema)
}

func decodeMetricset(input Input, schema *jsonschema.Schema) (transform.Transformable, error) {
	raw, err := validation.ValidateObject(input.Raw, schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate metricset")
	}

	md := metricsetDecoder{&utility.ManualDecoder{}}
	fieldName := field.Mapper(input.Config.HasShortFieldNames)

	e := metricset.Metricset{
		Samples:     md.decodeSamples(raw[fieldName("samples")], input.Config.HasShortFieldNames),
		Transaction: md.decodeTransaction(raw[fieldName(transactionKey)], input.Config.HasShortFieldNames),
		Span:        md.decodeSpan(raw[fieldName(spanKey)], input.Config.HasShortFieldNames),
		Timestamp:   md.TimeEpochMicro(raw, "timestamp"),
		Metadata:    input.Metadata,
	}

	if md.Err != nil {
		return nil, md.Err
	}

	if tags := utility.Prune(md.MapStr(raw, "tags")); len(tags) > 0 {
		e.Labels = tags
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = input.RequestTime
	}

	return &e, nil
}

type metricsetDecoder struct {
	*utility.ManualDecoder
}

func (md *metricsetDecoder) decodeSamples(input interface{}, hasShortFieldNames bool) []*metricset.Sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	fieldName := field.Mapper(hasShortFieldNames)
	inverseFieldName := field.InverseMapper(hasShortFieldNames)

	samples := make([]*metricset.Sample, len(raw))
	i := 0
	value := fieldName("value")
	for name, s := range raw {
		if s == nil {
			continue
		}
		sampleMap, ok := s.(map[string]interface{})
		if !ok {
			md.Err = fmt.Errorf("invalid sample: %s: %s", name, s)
			return nil
		}

		samples[i] = &metricset.Sample{
			Name:  inverseFieldName(name),
			Value: md.Float64(sampleMap, value),
		}
		if md.Err != nil {
			return nil
		}
		i++
	}
	return samples
}

func (md *metricsetDecoder) decodeSpan(input interface{}, hasShortFieldNames bool) *metricset.Span {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for span in metric event")
		return nil
	}

	fieldName := field.Mapper(hasShortFieldNames)
	return &metricset.Span{
		Type:    md.StringPtr(raw, fieldName("type")),
		Subtype: md.StringPtr(raw, fieldName("subtype")),
	}
}
func (md *metricsetDecoder) decodeTransaction(input interface{}, hasShortFieldNames bool) *metricset.Transaction {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for transaction in metric event")
		return nil
	}

	fieldName := field.Mapper(hasShortFieldNames)
	return &metricset.Transaction{
		Type: md.StringPtr(raw, fieldName("type")),
		Name: md.StringPtr(raw, fieldName("name")),
	}
}
