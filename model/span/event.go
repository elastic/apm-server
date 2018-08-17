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
	"time"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/common"
)

var (
	Metrics         = monitoring.Default.NewRegistry("apm-server.processor.span", monitoring.PublishExpvar)
	transformations = monitoring.NewInt(Metrics, "transformations")

	stacktraceCounter = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter      = monitoring.NewInt(Metrics, "frames")

	spanDocType = "span"

	processorEventEntry = common.MapStr{"name": "transaction", "event": spanDocType}

	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "span")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Event struct {
	Name       string
	Type       string
	Start      float64
	Duration   float64
	Context    common.MapStr
	Stacktrace m.Stacktrace

	Timestamp     time.Time
	TransactionId *string

	// new in v2
	HexId    *string
	ParentId *string

	// deprecated in v2
	Id     *int
	Parent *int
}

func V1DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	e, raw, err := decodeEvent(input, err)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e.Id = decoder.IntPtr(raw, "id")
	e.Parent = decoder.IntPtr(raw, "parent")
	return e, decoder.Err
}

func V2DecodeEvent(input interface{}, err error) (transform.Transformable, error) {
	e, raw, err := decodeEvent(input, err)
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	e.HexId = decoder.StringPtr(raw, "id")
	e.ParentId = decoder.StringPtr(raw, "parent_id")
	return e, decoder.Err
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
		Name:          decoder.String(raw, "name"),
		Type:          decoder.String(raw, "type"),
		Start:         decoder.Float64(raw, "start"),
		Duration:      decoder.Float64(raw, "duration"),
		Context:       decoder.MapStr(raw, "context"),
		Timestamp:     decoder.TimeRFC3339(raw, "timestamp"),
		TransactionId: decoder.StringPtr(raw, "transaction_id"),
	}
	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		event.Stacktrace = *stacktr
	}
	return &event, raw, err
}

func (s *Event) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if frames := len(s.Stacktrace); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}

	if s.Timestamp.IsZero() {
		s.Timestamp = tctx.RequestTime
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor":   processorEventEntry,
			spanDocType:   s.fields(tctx),
			"transaction": common.MapStr{"id": s.TransactionId},
			"context":     tctx.Metadata.MergeMinimal(s.Context),
		},
		Timestamp: s.Timestamp,
	}

	return []beat.Event{ev}
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
	utility.Add(tr, "hex_id", s.HexId)
	utility.Add(tr, "parent_id", s.ParentId)

	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "start", utility.MillisAsMicros(s.Start))
	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	st := s.Stacktrace.Transform(tctx)
	utility.Add(tr, "stacktrace", st)
	return tr
}
