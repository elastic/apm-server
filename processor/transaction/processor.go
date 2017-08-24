package transaction

import (
	"encoding/json"

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
		schema: pr.CreateSchema(transactionSchema, processorName),
	}
}

type processor struct {
	schema *jsonschema.Schema
}

func (p *processor) Validate(buf []byte) error {
	return pr.Validate(buf, p.schema)
}

func (p *processor) Transform(buf []byte) ([]beat.Event, error) {
	var pa payload
	err := json.Unmarshal(buf, &pa)
	if err != nil {
		return nil, err
	}

	return pa.transform(), nil
}

func (p *processor) Name() string {
	return processorName
}
