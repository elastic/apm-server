package error

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
	errorCounter = monitoring.NewInt(errorMetrics, "counter")
)

type payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"errors"`
}

func (pa *payload) transform(r *http.Request) []beat.Event {
	var events []beat.Event

	logp.Debug("error", "Transform error events: events=%d, app=%s, agent=%s:%s", len(pa.Events), pa.App.Name, pa.App.Agent.Name, pa.App.Agent.Version)

	// Inject system ip from the request
	if pa.System != nil {
		ip := utility.ExtractIP(r)
		pa.System.IP = &ip
	}

	errorCounter.Add(int64(len(pa.Events)))
	for _, e := range pa.Events {
		events = append(events, pr.CreateDoc(e.Mappings(pa)))
	}
	return events
}
