package error

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
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

type payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []Event
}

func decodeError(raw map[string]interface{}) (*payload, error) {
	if raw == nil {
		return nil, nil
	}
	pa := &payload{}
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
	pa.Events = make([]Event, len(errs))
	var event *Event
	err = decoder.Err
	for idx, errData := range errs {
		event, err = DecodeEvent(errData, err)
		if event != nil {
			pa.Events[idx] = *event
		}
	}
	return pa, err
}

func (pa *payload) transform(c pr.Config) []beat.Event {
	logp.NewLogger("transform").Debugf("Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)
	errorCounter.Add(int64(len(pa.Events)))

	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)

	config := m.Config{
		LibraryPattern:      c.LibraryPattern,
		ExcludeFromGrouping: c.ExcludeFromGrouping,
		SmapMapper:          c.SmapMapper,
	}
	var events []beat.Event
	for _, event := range pa.Events {
		context := context.Transform(event.Context)
		ev := beat.Event{
			Fields: common.MapStr{
				"processor":  processorEntry,
				errorDocType: event.Transform(config, pa.Service),
				"context":    context,
			},
			Timestamp: event.Timestamp,
		}
		if event.Transaction != nil {
			ev.Fields["transaction"] = common.MapStr{"id": event.Transaction.Id}
		}

		events = append(events, ev)
	}
	return events
}
