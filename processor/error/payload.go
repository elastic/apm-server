package error

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
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
	User    common.MapStr

	Events []Event `mapstructure:"errors"`
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
