package transaction

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transactionCounter = monitoring.NewInt(transactionMetrics, "counter")
	spanCounter        = monitoring.NewInt(transactionMetrics, "spans")
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

	logp.NewLogger("transaction").Debugf("Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
	for _, event := range pa.Events {

		events = append(events, pr.CreateDoc(event.Mappings(pa)))

		spanCounter.Add(int64(len(event.Spans)))
		for _, sp := range event.Spans {
			events = append(events, pr.CreateDoc(sp.Mappings(config, pa, event)))
		}
	}

	return events
}
