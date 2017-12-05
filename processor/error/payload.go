package error

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	errorCounter = monitoring.NewInt(errorMetrics, "counter")
)

type payload struct {
	Service m.Service
	System *m.System
	Events []Event `mapstructure:"errors"`
}

func (pa *payload) transform() []beat.Event {
	var events []beat.Event

	logp.Debug("error", "Transform error events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	errorCounter.Add(int64(len(pa.Events)))
	for _, e := range pa.Events {
		events = append(events, pr.CreateDoc(e.Mappings(pa)))
	}
	return events
}
