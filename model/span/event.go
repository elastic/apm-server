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

package span

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.span", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")

	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")

	spanDocType = "span"

	processorEntry = common.MapStr{"name": "transaction", "event": spanDocType}

	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "span")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
	Name       string
	Type       string
	Start      *float64
	Duration   float64
	Context    common.MapStr
	Stacktrace m.Stacktrace

	Timestamp     time.Time
	TransactionId string

	// new in v2
	HexId    string
	ParentId string
	TraceId  string

	// deprecated in v2
	Id     *int64
	Parent *int64

	v2Event bool
}

func V1DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	e, raw, err := decodeEvent(input, err)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e.Timestamp = decoder.TimeRFC3339(raw, "timestamp")
	e.Id = decoder.Int64Ptr(raw, "id")
	e.Parent = decoder.Int64Ptr(raw, "parent")
	if tid := decoder.StringPtr(raw, "transaction_id"); tid != nil {
		e.TransactionId = *tid
	}

	return e, decoder.Err
}

func V2DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	e, raw, err := decodeEvent(input, err)
	if err != nil {
		return nil, err
	}
	e.v2Event = true
	decoder := utility.ManualDecoder{}
	e.Timestamp = decoder.TimeEpochMicro(raw, "timestamp")
	e.HexId = decoder.String(raw, "id")
	e.ParentId = decoder.String(raw, "parent_id")
	e.TraceId = decoder.String(raw, "trace_id")
	e.TransactionId = decoder.String(raw, "transaction_id")
	if decoder.Err != nil {
		return nil, decoder.Err
	}

	// HexId must be a 64 bit hex encoded string. The id is set to the integer
	// converted value of the hexId
	idInt, err := hexToInt(e.HexId, 64)
	if err != nil {
		return nil, err
	}
	e.Id = &idInt

	// set parent to parentId
	id, err := hexToInt(e.ParentId, 64)
	if err != nil {
		return nil, err
	}
	e.Parent = &id

	return e, nil
}

var shift = uint64(math.Pow(2, 63))

func hexToInt(s string, bitSize int) (int64, error) {
	us, err := strconv.ParseUint(s, 16, bitSize)
	if err != nil {
		return 0, err
	}
	return int64(us - shift), nil
}

func decodeEvent(input interface{}, err error) (*Event, map[string]interface{}, error) {
	if err != nil {
		return nil, nil, err
	}
	if input == nil {
		return nil, nil, errors.New("Input missing for decoding Event")
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("Invalid type for span")
	}

	decoder := utility.ManualDecoder{}
	event := Event{
		Name:     decoder.String(raw, "name"),
		Type:     decoder.String(raw, "type"),
		Start:    decoder.Float64Ptr(raw, "start"),
		Duration: decoder.Float64(raw, "duration"),
		Context:  decoder.MapStr(raw, "context"),
	}
	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		event.Stacktrace = *stacktr
	}
	return &event, raw, err
}

func (e *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if frames := len(e.Stacktrace); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}

	fields := common.MapStr{
		"processor": processorEntry,
		spanDocType: e.fields(tctx),
		"context":   tctx.Metadata.MergeMinimal(e.Context),
	}
	utility.AddId(fields, "transaction", &e.TransactionId)
	utility.AddId(fields, "parent", &e.ParentId)
	utility.AddId(fields, "trace", &e.TraceId)

	timestamp := e.Timestamp
	if timestamp.IsZero() {
		timestamp = tctx.RequestTime
	}

	if e.v2Event {
		// adjust timestamp to be reqTime + start
		if e.Timestamp.IsZero() && e.Start != nil {
			timestamp = tctx.RequestTime.Add(time.Duration(float64(time.Millisecond) * *e.Start))
		}

		utility.Add(fields, "timestamp", utility.TimeAsMicros(timestamp))
	}

	return []beat.Event{
		beat.Event{
			Fields:    fields,
			Timestamp: timestamp,
		},
	}
}

func (s *Event) fields(tctx *transform.Context) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	// v1
	utility.Add(tr, "id", s.Id)
	utility.Add(tr, "parent", s.Parent)
	// v2
	if s.HexId != "" {
		utility.Add(tr, "hex_id", s.HexId)
	}

	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)

	if s.Start != nil {
		utility.Add(tr, "start", utility.MillisAsMicros(*s.Start))
	}

	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))

	st := s.Stacktrace.Transform(tctx)
	utility.Add(tr, "stacktrace", st)
	return tr
}
