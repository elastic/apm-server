package transaction

import (
	"io"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/publisher/beat"

	"github.com/santhosh-tekuri/jsonschema"
)

func init() {
	pr.Registry.AddProcessor("/v1/transactions", NewProcessor())
}

const (
	processorName = "transaction"
)

func NewProcessor() pr.Processor {
	return &processor{
		payload: &Payload{},
		schema:  pr.CreateSchema(transactionSchema, processorName),
	}
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
