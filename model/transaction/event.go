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

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/common"
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
)

var (
	Metrics = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)

	spanCounter     = monitoring.NewInt(Metrics, "spans")
	transformations = monitoring.NewInt(Metrics, "transformations")

	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
)

var (
	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "transaction")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
	Id        string
	Type      string
	Name      *string
	Result    *string
	Duration  float64
	Timestamp time.Time
	Context   common.MapStr
	Marks     common.MapStr
	Sampled   *bool
	SpanCount SpanCount
	Spans     []*span.Span
}
type SpanCount struct {
	Dropped Dropped
}
type Dropped struct {
	Total *int
}

func DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for transaction event")
	}
	decoder := utility.ManualDecoder{}
	e := Event{
		Id:        decoder.String(raw, "id"),
		Type:      decoder.String(raw, "type"),
		Name:      decoder.StringPtr(raw, "name"),
		Result:    decoder.StringPtr(raw, "result"),
		Duration:  decoder.Float64(raw, "duration"),
		Timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
		Context:   decoder.MapStr(raw, "context"),
		Marks:     decoder.MapStr(raw, "marks"),
		Sampled:   decoder.BoolPtr(raw, "sampled"),
		SpanCount: SpanCount{Dropped: Dropped{Total: decoder.IntPtr(raw, "total", "span_count", "dropped")}},
	}
	err = decoder.Err
	var transformable transform.Transformable
	spans := decoder.InterfaceArr(raw, "spans")
	e.Spans = make([]*span.Span, len(spans))
	for idx, rawSpan := range spans {
		transformable, err = span.DecodeSpan(rawSpan, err)
		sp := transformable.(*span.Span)

		if sp.Timestamp.IsZero() {
			sp.Timestamp = e.Timestamp
		}

		if sp.TransactionId == "" {
			sp.TransactionId = e.Id
		}

		e.Spans[idx] = sp
	}
	return &e, err
}

func (t *Event) fields(tctx *transform.Context) common.MapStr {
	tx := common.MapStr{"id": t.Id}
	utility.Add(tx, "name", t.Name)
	utility.Add(tx, "duration", utility.MillisAsMicros(t.Duration))
	utility.Add(tx, "type", t.Type)
	utility.Add(tx, "result", t.Result)
	utility.Add(tx, "marks", t.Marks)

	if t.Sampled == nil {
		utility.Add(tx, "sampled", true)
	} else {
		utility.Add(tx, "sampled", t.Sampled)
	}

	if t.SpanCount.Dropped.Total != nil {
		s := common.MapStr{
			"dropped": common.MapStr{
				"total": *t.SpanCount.Dropped.Total,
			},
		}
		utility.Add(tx, "span_count", s)
	}
	return tx
}

func (e *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	events := []beat.Event{}
	ev := beat.Event{
		Fields: common.MapStr{
			"processor":        processorTransEntry,
			transactionDocType: e.fields(tctx),
			"context":          tctx.Metadata.Merge(e.Context),
		},
		Timestamp: e.Timestamp,
	}
	events = append(events, ev)

	spanCounter.Add(int64(len(e.Spans)))
	for spIdx := 0; spIdx < len(e.Spans); spIdx++ {
		events = append(events, e.Spans[spIdx].Transform(tctx)...)
		e.Spans[spIdx] = nil
	}
	return events
}
