package processor

import (
	"github.com/elastic/apm-server/config"
	"github.com/elastic/beats/libbeat/beat"
)

type NewProcessor func(conf config.Config) Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Transform(map[string]interface{}) ([]beat.Event, error)
	Name() string
}
