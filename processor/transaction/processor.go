package transaction

import (
	"encoding/json"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
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
	processorName = "transaction"
)

var schema = pr.CreateSchema(transactionSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: schema}
}

type processor struct {
	schema *jsonschema.Schema
}

func (p *processor) Validate(buf []byte) error {
	validationCount.Inc()
	err := pr.Validate(buf, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(buf []byte) ([]beat.Event, error) {
	var pa payload
	transformations.Inc()
	err := json.Unmarshal(buf, &pa)
	if err != nil {
		return nil, err
	}

	return pa.transform(), nil
}

func (p *processor) Name() string {
	return processorName
}
