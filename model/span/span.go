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

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/santhosh-tekuri/jsonschema"

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

	processorSpanEntry = common.MapStr{"name": "transaction", "event": spanDocType}

	cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "transaction")
)

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type Span struct {
	Id         *int
	Name       string
	Type       string
	Start      float64
	Duration   float64
	Context    common.MapStr
	Parent     *int
	Stacktrace m.Stacktrace

	Timestamp     time.Time
	TransactionId string
}

func DecodeSpan(input interface{}, err error) (transform.Transformable, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for span")
	}
	decoder := utility.ManualDecoder{}
	sp := Span{
		Id:       decoder.IntPtr(raw, "id"),
		Name:     decoder.String(raw, "name"),
		Type:     decoder.String(raw, "type"),
		Start:    decoder.Float64(raw, "start"),
		Duration: decoder.Float64(raw, "duration"),
		Context:  decoder.MapStr(raw, "context"),
		Parent:   decoder.IntPtr(raw, "parent"),
	}
	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		sp.Stacktrace = *stacktr
	}
	return &sp, err
}

func (s *Span) Transform(tctx *transform.Context) []beat.Event {
	transformations.Inc()
	if frames := len(s.Stacktrace); frames > 0 {
		stacktraceCounter.Inc()
		frameCounter.Add(int64(frames))
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor":   processorSpanEntry,
			spanDocType:   s.fields(tctx),
			"transaction": common.MapStr{"id": s.TransactionId},
			"context":     tctx.Metadata.MergeMinimal(s.Context),
		},
		Timestamp: s.Timestamp,
	}

	return []beat.Event{ev}
}

func (s *Span) fields(tctx *transform.Context) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	utility.Add(tr, "id", s.Id)
	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "start", utility.MillisAsMicros(s.Start))
	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	utility.Add(tr, "parent", s.Parent)
	st := s.Stacktrace.Transform(tctx)
	utility.Add(tr, "stacktrace", st)
	return tr
}
