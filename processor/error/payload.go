package error

import (
	"errors"

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

	Events []Event
}

func (pa *payload) decode(raw map[string]interface{}) error {
	if err := pa.Service.Decode(raw["service"]); err != nil {
		return err
	}
	if pa.System == nil {
		pa.System = &m.System{}
	}
	if err := pa.System.Decode(raw["system"]); err != nil {
		return err
	}
	if pa.Process == nil {
		pa.Process = &m.Process{}
	}
	if err := pa.Process.Decode(raw["process"]); err != nil {
		return err
	}
	if pa.User == nil {
		pa.User = &m.User{}
	}
	if err := pa.User.Decode(raw["user"]); err != nil {
		return err
	}

	df := utility.DataFetcher{}
	errs := df.InterfaceArr(raw, "errors")
	if errs == nil {
		pa.Events = make([]Event, 0)
		return df.Err
	}
	pa.Events = make([]Event, len(errs))

	for idx, err := range errs {
		errData, ok := err.(map[string]interface{})
		if !ok {
			return errors.New("Invalid type for error")
		}
		event := Event{}
		if err := event.decode(errData); err != nil {
			return err
		}
		pa.Events[idx] = event
	}
	return df.Err
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event
	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)

	logp.NewLogger("transform").Debugf("Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	errorCounter.Add(int64(len(pa.Events)))
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
