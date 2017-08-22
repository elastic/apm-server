package error

import (
	pr "github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/beat"
)

type payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"errors"`
}

func (pa *payload) Transform() []beat.Event {
	var events []beat.Event

	for _, e := range pa.Events {
		events = append(events, pr.CreateDoc(e.Mappings(pa)))
	}
	return events
}
