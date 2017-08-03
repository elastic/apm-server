package transaction

import (
	pr "github.com/elastic/apm-server/processor"
	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

type Payload struct {
	App    m.App     `json:"app"`
	System *m.System `json:"system"`
	Events []Event   `json:"transactions"`
}

func (pa *Payload) Transform() []common.MapStr {
	var docs []common.MapStr
	for _, tx := range pa.Events {

		docs = append(docs, pr.CreateDoc(tx.Mappings(pa)))

		for _, tr := range tx.Traces {
			docs = append(docs, pr.CreateDoc(tr.Mappings(pa, tx)))
		}
	}

	return docs
}
