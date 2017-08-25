package transaction

import (
	pr "github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transactionCounter = monitoring.NewInt(transactionMetrics, "counter")
	traceCounter       = monitoring.NewInt(transactionMetrics, "traces")
)

type payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"transactions"`
}

func (pa *payload) transform() []beat.Event {
	var events []beat.Event

	logp.Debug("transaction", "Transform transaction events: events=%d, app=%s, agent=%s:%s", len(pa.Events), pa.App.Name, pa.App.Agent.Name, pa.App.Agent.Version)

	transactionCounter.Add(int64(len(pa.Events)))
	for _, tx := range pa.Events {

		events = append(events, pr.CreateDoc(tx.Mappings(pa)))

		traceCounter.Add(int64(len(tx.Traces)))
		for _, tr := range tx.Traces {
			events = append(events, pr.CreateDoc(tr.Mappings(pa, tx)))
		}
	}

	return events
}
