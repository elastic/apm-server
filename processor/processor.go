package processor

import (
	"github.com/elastic/apm-server/config"
	"github.com/elastic/beats/libbeat/beat"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Decode(map[string]interface{}) (Payload, error)
	Name() string
}
type Payload interface {
	Transform(config.Config) []beat.Event
}

type Decoder interface {
	DecodePayload(map[string]interface{}) (*Payload, error)
}
