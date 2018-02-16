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
	m.Service
	*m.System
	*m.Process
	Events  []Event
	User    map[string]interface{}
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event
	service := pa.Service.Transform()
	system := pa.System.Transform()
	process := pa.Process.Transform()

	logp.NewLogger("transform").Debugf("Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.AgentName, pa.Service.Agent.AgentVersion)

	errorCounter.Add(int64(len(pa.Events)))
	for _, event := range pa.Events {
		context := event.contextTransform(pa)
		if context == nil {
			context = common.MapStr{}
		}
		utility.Add(context, "service", service)
		utility.Add(context, "system", system)
		utility.Add(context, "process", process)

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
