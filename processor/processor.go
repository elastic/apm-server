package processor

import (
	"io"

	m "github.com/elastic/apm-server/processor/model"
	"github.com/elastic/beats/libbeat/common"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(io.Reader) error
	Payload
	Name() string
}

type Payload interface {
	Transform() []common.MapStr
}

func CreateDoc(baseMappings []m.SMapping, docMappings []m.FMapping) common.MapStr {
	doc := common.MapStr{}
	for _, mapping := range baseMappings {
		doc.Put(mapping.Key, mapping.Value)
	}
	for _, mapping := range docMappings {
		if out := mapping.Apply(); out != nil {
			doc.Put(mapping.Key, out)
		}
	}
	return doc
}
