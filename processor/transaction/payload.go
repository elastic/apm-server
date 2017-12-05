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
	System *m.System
	Events []Event `mapstructure:"transactions"`
}

func (pa *payload) transform() []beat.Event {
	var events []beat.Event

	logp.Debug("transaction", "Transform transaction events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
	for _, tx := range pa.Events {

		events = append(events, pr.CreateDoc(tx.Mappings(pa)))

		spanCounter.Add(int64(len(tx.Spans)))
		for _, sp := range tx.Spans {
			events = append(events, pr.CreateDoc(sp.Mappings(pa, tx)))
		}
	}

	return events
}
