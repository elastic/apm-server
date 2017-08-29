package processor

import (
	"github.com/elastic/beats/libbeat/logp"

	"time"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type NewProcessor func() Processor

type Processor interface {
	Validate([]byte) error
	Transform([]byte) ([]beat.Event, error)
	Name() string
}

func CreateDocUnsafe(strTime string, docMappings []m.DocMapping) beat.Event {
	timestamp, err := utility.ParseTime(strTime)
	if err != nil {
		logp.Err("Unable to parse timestamp %s: %s", strTime, err)
	}
	return CreateDoc(timestamp, docMappings)
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
