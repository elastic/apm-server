package processor

import (
	"io"

	"github.com/elastic/beats/libbeat/logp"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(io.Reader) error
	Transform() []beat.Event
	Name() string
}

func CreateDoc(strTime string, docMappings []m.DocMapping) beat.Event {
	doc := common.MapStr{}
	for _, mapping := range docMappings {
		if out := mapping.Apply(); out != nil {
			doc.Put(mapping.Key, out)
		}
	}

	timestamp, err := utility.ParseTime(strTime)
	if err != nil {
		logp.Err("Unable to parse timestamp %s: %s", strTime, err)
	}

	return beat.Event{
		Fields:    doc,
		Timestamp: timestamp,
	}
}
