package error

import (
	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	errorCounter   = monitoring.NewInt(errorMetrics, "counter")
	processorEntry = common.MapStr{"name": processorName, "event": errorDocType}
)

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Event
}

func DecodePayload(raw map[string]interface{}) (*Payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := Payload{}

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
	errs := decoder.InterfaceArr(raw, "errors")
	pa.Events = make([]*Event, len(errs))
	err = decoder.Err
	for idx, errData := range errs {
		pa.Events[idx], err = DecodeEvent(errData, err)
	}
	return &pa, err
}

func (pa *Payload) Transform(conf config.Config) []beat.Event {
	logp.NewLogger("transform").Debugf("Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)
	errorCounter.Add(int64(len(pa.Events)))

	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)

	var events []beat.Event
	for idx := 0; idx < len(pa.Events); idx++ {
		event := pa.Events[idx]
		context := context.Transform(event.Context)
		var zeroTime = time.Time{}
		if event.Timestamp == zeroTime {
			event.Timestamp = time.Now()
		}
		ev := beat.Event{
			Fields: common.MapStr{
				"processor":  processorEntry,
				errorDocType: event.Transform(conf, pa.Service),
				"context":    context,
			},
			Timestamp: event.Timestamp,
		}
		if event.Transaction != nil {
			ev.Fields["transaction"] = common.MapStr{"id": event.Transaction.Id}
		}
		events = append(events, ev)
		pa.Events[idx] = nil
	}
	return events
}
