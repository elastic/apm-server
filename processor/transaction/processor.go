package transaction

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/santhosh-tekuri/jsonschema"
)

var (
	transactionMetrics = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)
	decodingCount      = monitoring.NewInt(transactionMetrics, "decoding.count")
	decodingError      = monitoring.NewInt(transactionMetrics, "decoding.errors")
	validationCount    = monitoring.NewInt(transactionMetrics, "validation.count")
	validationError    = monitoring.NewInt(transactionMetrics, "validation.errors")
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
	spanDocType        = "span"
)

var schema = pr.CreateSchema(transactionSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: schema}
}

type processor struct {
	schema *jsonschema.Schema
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	err := pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Decode(raw map[string]interface{}) (pr.Payload, error) {
	decodingCount.Inc()
	pa, err := DecodePayload(raw)
	if err != nil {
		decodingError.Inc()
		return nil, err
	}
	return pa, nil
}
