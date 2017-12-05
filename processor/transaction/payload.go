package transaction

import (
	"net/http"

	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transactionCounter = monitoring.NewInt(transactionMetrics, "counter")
	spanCounter        = monitoring.NewInt(transactionMetrics, "spans")
)

type payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"transactions"`
}

func (pa *payload) transform(r *http.Request) []beat.Event {
	var events []beat.Event

	logp.Debug("transaction", "Transform transaction events: events=%d, app=%s, agent=%s:%s", len(pa.Events), pa.App.Name, pa.App.Agent.Name, pa.App.Agent.Version)

	// Inject system ip from the request
	if pa.System != nil {
		ip := utility.ExtractIP(r)
		pa.System.IP = &ip
	}

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
