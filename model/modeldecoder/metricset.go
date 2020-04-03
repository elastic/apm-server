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

	"github.com/pkg/errors"

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

var metricsetSchema = validation.CreateSchema(schema.ModelSchema, "metricset")

type metricsetDecoder struct {
	*utility.ManualDecoder
}

func DecodeMetricset(input Input) (transform.Transformable, error) {
	raw, err := validation.ValidateObject(input.Raw, metricsetSchema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate metricset")
	}

	md := metricsetDecoder{&utility.ManualDecoder{}}
	e := metricset.Metricset{
		Samples:     md.decodeSamples(raw["samples"]),
		Transaction: md.decodeTransaction(raw[transactionKey]),
		Span:        md.decodeSpan(raw[spanKey]),
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

func (md *metricsetDecoder) decodeSamples(input interface{}) []*metricset.Sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	samples := make([]*metricset.Sample, len(raw))
	i := 0
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
			Name:  name,
			Value: md.Float64(sampleMap, "value"),
		}
		if md.Err != nil {
			return nil
		}
		i++
	}
	return samples
}

func (md *metricsetDecoder) decodeSpan(input interface{}) *metricset.Span {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for span in metric event")
		return nil
	}

	return &metricset.Span{
		Type:    md.StringPtr(raw, "type"),
		Subtype: md.StringPtr(raw, "subtype"),
	}
}
func (md *metricsetDecoder) decodeTransaction(input interface{}) *metricset.Transaction {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for transaction in metric event")
		return nil
	}

	return &metricset.Transaction{
		Type: md.StringPtr(raw, "type"),
		Name: md.StringPtr(raw, "name"),
	}
}
