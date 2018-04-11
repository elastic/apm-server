package error

import (
	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	errorMetrics    = monitoring.Default.NewRegistry("apm-server.processor.error")
	validationCount = monitoring.NewInt(errorMetrics, "validation.count")
	validationError = monitoring.NewInt(errorMetrics, "validation.errors")
	transformations = monitoring.NewInt(errorMetrics, "transformations")
)

const (
	processorName = "error"
	errorDocType  = "error"
)

var schema = pr.CreateSchema(errorSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: schema}
}

func (p *processor) Name() string {
	return processorName
}

type processor struct {
	schema *jsonschema.Schema
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
	transformations.Inc()
	pa, err := DecodePayload(raw)
	if err != nil {
		return nil, err
	}
	return pa, nil
}
