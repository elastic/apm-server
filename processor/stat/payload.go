package stat

import (
	m "github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	statMetrics = monitoring.Default.NewRegistry("apm-server.processor.stat")
	statCounter = monitoring.NewInt(statMetrics, "counter")
)

type payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	Events  []Event `mapstructure:"stats"`
}

func (pa *payload) transform(config *pr.Config) []beat.Event {
	var events []beat.Event

	logp.Debug("stat", "Transform stat events: events=%d, service=%s, agent=%s:%s", len(pa.Events), pa.Service.Name, pa.Service.Agent.Name, pa.Service.Agent.Version)

	statCounter.Add(int64(len(pa.Events)))
	for _, event := range pa.Events {
		events = append(events, pr.CreateDoc(event.Mappings(pa)))
	}

	return events
}
