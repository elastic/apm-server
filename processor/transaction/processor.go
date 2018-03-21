package transaction

import (
	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/santhosh-tekuri/jsonschema"
)

var (
	transactionMetrics = monitoring.Default.NewRegistry("apm-server.processor.transaction")
	transformations    = monitoring.NewInt(transactionMetrics, "transformations")
	validationCount    = monitoring.NewInt(transactionMetrics, "validation.count")
	validationError    = monitoring.NewInt(transactionMetrics, "validation.errors")
)

const (
	eventName          = "processor"
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

func (p *processor) Decode(config config.Config, raw map[string]interface{}) (pr.Payload, error) {
	transformations.Inc()
	pa, err := DecodePayload(config, raw)
	if err != nil {
		return nil, err
	}
	return pa, nil
}
