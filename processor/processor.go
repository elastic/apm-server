package processor

import (
	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/beat"
)

type NewProcessor func() Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Decode(map[string]interface{}) (model.TransformableBatch, error)
	Name() string
}

type Decoder interface {
	DecodePayload(map[string]interface{}) (model.Transformable, error)
}

type Payload interface {
	Transform(config.TransformConfig, *model.TransformContext) []beat.Event
}
