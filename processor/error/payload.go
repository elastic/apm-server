package error

import (
	pr "github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

type Payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"errors"`
}

func (pa *Payload) Transform() []common.MapStr {
	var events []common.MapStr

	for _, e := range pa.Events {
		events = append(events, pr.CreateDoc(e.Mappings(pa)))
	}
	return events
}
