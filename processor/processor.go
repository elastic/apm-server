package processor

import (
	"io"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(io.Reader) error
	Payload
	Name() string
}

type Payload interface {
	Transform() []beat.Event
}

func CreateDoc(strTime string, baseMappings []m.SMapping, docMappings []m.FMapping) beat.Event {
	doc := common.MapStr{}
	for _, mapping := range baseMappings {
		doc.Put(mapping.Key, mapping.Value)
	}
	for _, mapping := range docMappings {
		if out := mapping.Apply(); out != nil {
			doc.Put(mapping.Key, out)
		}
	}

	// This assumes JSON Spec has already validated the timestamp to be the correct format.
	timestamp, err := utility.ParseTime(strTime)
	if err != nil {
		logp.Err("Unable to parse timestamp %s: %s", strTime, err)
	}
	return beat.Event{
		Fields:    doc,
		Timestamp: timestamp,
	}
}
