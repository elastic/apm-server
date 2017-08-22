package error

import (
	"io"

	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
)

func init() {
	pr.Registry.AddProcessor("/v1/errors", NewProcessor())
}

const (
	processorName = "error"
)

func NewProcessor() pr.Processor {
	schema := pr.CreateSchema(errorSchema, processorName)
	return &processor{
		&payload{},
		schema}
}

type processor struct {
	payload pr.Payload
	schema  *jsonschema.Schema
}

func (p *processor) Transform() []beat.Event {
	return p.payload.Transform()
}

func (p *processor) Validate(reader io.Reader) error {
	return pr.Validate(reader, p.schema, p.payload)
}

func (p *processor) Name() string {
	return processorName
}
