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

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")
	processorEntry  = common.MapStr{"name": processorName, "event": transactionDocType}
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

	ParentId *string
	TraceId  string
}

type SpanCount struct {
	Dropped *int
	Started *int
}

func DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	if err != nil {
		return nil, err
	}
	if input == nil {
		return nil, errors.New("Input missing for decoding Event")
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
		Context:   decoder.MapStr(raw, "context"),
		Marks:     decoder.MapStr(raw, "marks"),
		Sampled:   decoder.BoolPtr(raw, "sampled"),
		Timestamp: decoder.TimeEpochMicro(raw, "timestamp"),
		SpanCount: SpanCount{
			Dropped: decoder.IntPtr(raw, "dropped", "span_count"),
			Started: decoder.IntPtr(raw, "started", "span_count")},
		ParentId: decoder.StringPtr(raw, "parent_id"),
		TraceId:  decoder.String(raw, "trace_id"),
	}

	if decoder.Err != nil {
		return nil, decoder.Err
	}

	return &e, nil
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

	if t.SpanCount.Dropped != nil || t.SpanCount.Started != nil {
		spanCount := common.MapStr{}

		if t.SpanCount.Dropped != nil {
			utility.Add(spanCount, "dropped", common.MapStr{"total": *t.SpanCount.Dropped})
		}
		if t.SpanCount.Started != nil {
			utility.Add(spanCount, "started", *t.SpanCount.Started)
		}
		utility.Add(tx, "span_count", spanCount)
	}
	return tx
}

func (e *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	events := []beat.Event{}

	if e.Timestamp.IsZero() {
		e.Timestamp = tctx.RequestTime
	}

	fields := common.MapStr{
		"processor":        processorEntry,
		transactionDocType: e.fields(tctx),
		"context":          tctx.Metadata.Merge(e.Context),
	}
	utility.AddId(fields, "parent", e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)
	utility.Add(fields, "timestamp", utility.TimeAsMicros(e.Timestamp))

	events = append(events, beat.Event{Fields: fields, Timestamp: e.Timestamp})

	return events
}
