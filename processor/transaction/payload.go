package transaction

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
	transactionCounter  = monitoring.NewInt(transactionMetrics, "counter")
	spanCounter         = monitoring.NewInt(transactionMetrics, "spans")
	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}
)

type payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	Events  []Event `mapstructure:"transactions"`
	User    map[string]interface{}
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event
	spanService := pa.Service.MinimalTransform()
	service := pa.Service.Transform()
	system := pa.System.Transform()
	process := pa.Process.Transform()

	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
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
				"processor":        processorTransEntry,
				transactionDocType: event.Transform(),
				"context":          context,
			},
			Timestamp: event.Timestamp,
		}
		events = append(events, ev)

		trId := common.MapStr{"id": event.Id}
		spanCounter.Add(int64(len(event.Spans)))
		for _, sp := range event.Spans {
			c := sp.Context
			if c == nil && spanService != nil {
				c = common.MapStr{}
			}
			utility.Add(c, "service", spanService)

			ev := beat.Event{
				Fields: common.MapStr{
					"processor":   processorSpanEntry,
					spanDocType:   sp.Transform(config, pa.Service),
					"transaction": trId,
					"context":     c,
				},
				Timestamp: event.Timestamp,
			}
			events = append(events, ev)
		}
	}

	return events
}
