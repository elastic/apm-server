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
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/model"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
	spanDocType        = "span"
)

var (
	transformations    = monitoring.NewInt(Metrics, "transformations")
	transactionCounter = monitoring.NewInt(Metrics, "transactions")
	spanCounter        = monitoring.NewInt(Metrics, "spans")
	stacktraceCounter  = monitoring.NewInt(Metrics, "stacktraces")
	frameCounter       = monitoring.NewInt(Metrics, "frames")

	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}

	Metrics = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)

	cachedSchema = validation.CreateSchema(schema.PayloadSchema, "transaction")
)

func PayloadSchema() *jsonschema.Schema {
	return cachedSchema
}

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Event
}

func DecodePayload(raw map[string]interface{}) (model.Payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := &Payload{}

	var err error
	service, err := m.DecodeService(raw["service"], err)
	if service != nil {
		pa.Service = *service
	}
	pa.System, err = m.DecodeSystem(raw["system"], err)
	pa.Process, err = m.DecodeProcess(raw["process"], err)
	pa.User, err = m.DecodeUser(raw["user"], err)
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	txs := decoder.InterfaceArr(raw, "transactions")
	err = decoder.Err
	pa.Events = make([]*Event, len(txs))
	for idx, tx := range txs {
		pa.Events[idx], err = DecodeEvent(tx, err)
	}
	return pa, err
}

func (pa *Payload) Transform(conf config.Config) []beat.Event {
	transformations.Inc()
	transactionCounter.Add(int64(len(pa.Events)))
	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)
	spanContext := NewSpanContext(&pa.Service)

	var events []beat.Event
	for idx := 0; idx < len(pa.Events); idx++ {
		event := pa.Events[idx]
		ev := beat.Event{
			Fields: common.MapStr{
				"processor":        processorTransEntry,
				transactionDocType: event.Transform(),
				"context":          context.Transform(event.Context),
			},
			Timestamp: event.Timestamp,
		}
		events = append(events, ev)

		trId := common.MapStr{"id": event.Id}
		spanCounter.Add(int64(len(event.Spans)))
		for spIdx := 0; spIdx < len(event.Spans); spIdx++ {
			sp := event.Spans[spIdx]
			if frames := len(sp.Stacktrace); frames > 0 {
				stacktraceCounter.Inc()
				frameCounter.Add(int64(frames))
			}
			ev := beat.Event{
				Fields: common.MapStr{
					"processor":   processorSpanEntry,
					spanDocType:   sp.Transform(conf, pa.Service),
					"transaction": trId,
					"context":     spanContext.Transform(sp.Context),
				},
				Timestamp: event.Timestamp,
			}
			events = append(events, ev)
			event.Spans[spIdx] = nil
		}
		pa.Events[idx] = nil
	}

	return events
}
