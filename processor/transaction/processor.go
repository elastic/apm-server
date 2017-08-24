package transaction

import (
	"io"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"

	"github.com/santhosh-tekuri/jsonschema"
)

func init() {
	pr.Registry.AddProcessor(Endpoint, NewProcessor())
}

const (
	Endpoint      = "/v1/transactions"
	processorName = "transaction"
)

func NewProcessor() pr.Processor {
	return &processor{
		payload: &payload{},
		schema:  pr.CreateSchema(transactionSchema, processorName),
	}
}

type processor struct {
	payload *payload
	schema  *jsonschema.Schema
}

func (p *processor) Transform() []beat.Event {
	return p.payload.transform()
}

func (p *processor) Validate(reader io.Reader) error {
	return pr.Validate(reader, p.schema, p.payload)
}

func (p *processor) Name() string {
	return processorName
}
