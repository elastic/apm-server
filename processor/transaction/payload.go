package transaction

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
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
	Context common.MapStr
	User    common.MapStr
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event
	context := m.NewContext(&pa.Service, pa.Process, pa.System, pa.User)
	spanContext := NewSpanContext(&pa.Service)

	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
	for _, event := range pa.Events {

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
		for _, sp := range event.Spans {
			ev := beat.Event{
				Fields: common.MapStr{
					"processor":   processorSpanEntry,
					spanDocType:   sp.Transform(config, pa.Service),
					"transaction": trId,
					"context":     spanContext.Transform(sp.Context),
				},
				Timestamp: event.Timestamp,
			}
			events = append(events, ev)
		}
	}

	return events
}
