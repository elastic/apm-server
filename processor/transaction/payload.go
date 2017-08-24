package transaction

import (
	pr "github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
)

type payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"transactions"`
}

func (pa *payload) transform() []beat.Event {
	var events []beat.Event

	logp.Debug("transaction", "Transform transaction events: events=%d, app=%s, agent=%s:%s", len(pa.Events), pa.App.Name, pa.App.Agent.Name, pa.App.Agent.Version)

	for _, tx := range pa.Events {

		events = append(events, pr.CreateDoc(tx.Mappings(pa)))

		for _, tr := range tx.Traces {
			events = append(events, pr.CreateDoc(tr.Mappings(pa, tx)))
		}
	}

	return events
}
