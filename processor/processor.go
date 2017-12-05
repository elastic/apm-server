package processor

import (
	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Transform(interface{}) ([]beat.Event, error)
	Name() string
}

func CreateDoc(timestamp time.Time, docMappings []m.DocMapping) beat.Event {
	doc := common.MapStr{}
	for _, mapping := range docMappings {
		if out := mapping.Apply(); out != nil {
			doc.Put(mapping.Key, out)
		}
	}

	return beat.Event{
		Fields:    doc,
		Timestamp: timestamp,
	}
}
