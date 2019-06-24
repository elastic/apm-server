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

package metricset

import (
	"errors"
	"fmt"
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metricset/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName  = "metric"
	docType        = "metric"
	transactionKey = "transaction"
	spanKey        = "span"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.metric", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": docType}

	knownSpanSampleAttrs = []string{
		"span.self_time.count",
		"span.self_time.sum.us",
	}
	knownTransactionSampleAttrs = []string{
		"transaction.self_time.count",
		"transaction.self_time.sum.us",
		"transaction.duration.sum.us",
		"transaction.breakdown.count",
	}
)

var cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "metricset")

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Sample struct {
	Name  string
	Value float64
}

// Transaction provides enough information to connect a metricset to the related kind of transactions
type Transaction struct {
	Name *string
	Type *string
}

// Span provides enough information to connect a metricset to the related kind of spans
type Span struct {
	Type    *string
	Subtype *string
}

type Metricset struct {
	Samples     []*Sample
	Labels      common.MapStr
	Transaction *Transaction
	Span        *Span
	Timestamp   time.Time
}

type metricsetDecoder struct {
	*utility.ManualDecoder
}

func DecodeEvent(input interface{}, _ model.Config, err error) (transform.Transformable, error) {
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, errors.New("no data for metric event")
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for metric event")
	}

	md := metricsetDecoder{&utility.ManualDecoder{}}
	e := Metricset{
		Samples:     md.decodeSamples(raw["samples"]),
		Transaction: md.decodeTransaction(raw[transactionKey]),
		Span:        md.decodeSpan(raw[spanKey]),
		Timestamp:   md.TimeEpochMicro(raw, "timestamp"),
	}

	if md.Err != nil {
		return nil, md.Err
	}

	if tags := utility.Prune(md.MapStr(raw, "tags")); len(tags) > 0 {
		e.Labels = tags
	}

	return &e, nil
}

func (md *metricsetDecoder) decodeSamples(input interface{}) []*Sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	samples := make([]*Sample, len(raw))
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

		samples[i] = &Sample{
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

func (md *metricsetDecoder) decodeSpan(input interface{}) *Span {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for span in metric event")
		return nil
	}

	return &Span{
		Type:    md.StringPtr(raw, "type"),
		Subtype: md.StringPtr(raw, "subtype"),
	}
}

func (md *metricsetDecoder) decodeTransaction(input interface{}) *Transaction {
	if input == nil {
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for transaction in metric event")
		return nil
	}

	return &Transaction{
		Type: md.StringPtr(raw, "type"),
		Name: md.StringPtr(raw, "name"),
	}
}

func (s *Span) fields() common.MapStr {
	if s == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "type", s.Type)
	utility.Set(fields, "subtype", s.Subtype)
	return fields
}

func (t *Transaction) fields() common.MapStr {
	if t == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "type", t.Type)
	utility.Set(fields, "name", t.Name)
	return fields
}

func (me *Metricset) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if me == nil {
		return nil
	}

	sampleSpanFields := common.MapStr{"span": common.MapStr{}}
	sampleTransactionFields := common.MapStr{"transaction": common.MapStr{}}

	fields := common.MapStr{}
	for _, sample := range me.Samples {
		if utility.Contains(sample.Name, knownSpanSampleAttrs) {
			sampleSpanFields.Put(sample.Name, sample.Value)
		} else if utility.Contains(sample.Name, knownTransactionSampleAttrs) {
			sampleTransactionFields.Put(sample.Name, sample.Value)
		} else if _, err := fields.Put(sample.Name, sample.Value); err != nil {
			logp.NewLogger("transform").Warnf("failed to transform sample %#v", sample)
			continue
		}
	}

	transactionFields := me.Transaction.fields()
	transactionFields.DeepUpdate(sampleTransactionFields["transaction"].(common.MapStr))

	spanFields := me.Span.fields()
	spanFields.DeepUpdate(sampleSpanFields["span"].(common.MapStr))

	fields["processor"] = processorEntry
	tctx.Metadata.Set(fields)

	// merges with metadata labels, overrides conflicting keys
	utility.DeepUpdate(fields, "labels", me.Labels)
	utility.DeepUpdate(fields, transactionKey, transactionFields)
	utility.DeepUpdate(fields, spanKey, spanFields)

	if me.Timestamp.IsZero() {
		me.Timestamp = tctx.RequestTime
	}

	return []beat.Event{
		{
			Fields:    fields,
			Timestamp: me.Timestamp,
		},
	}
}
