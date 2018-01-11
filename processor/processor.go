package processor

import (
	"regexp"
	"time"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type NewProcessor func(conf *Config) Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Transform(interface{}) ([]beat.Event, error)
	Name() string
}

type Config struct {
	SmapMapper     sourcemap.Mapper
	LibraryPattern *regexp.Regexp
}

func CreateDoc(timestamp time.Time, docMappings []utility.DocMapping) beat.Event {
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
