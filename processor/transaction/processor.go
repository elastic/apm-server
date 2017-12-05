package transaction

import (
	"github.com/mitchellh/mapstructure"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/utility"
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

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	err := pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(raw interface{}) ([]beat.Event, error) {
	var pa payload
	transformations.Inc()

	decoder, _ := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: utility.RFC3339DecoderHook,
			Result:     &pa,
		},
	)
	err := decoder.Decode(raw)
	if err != nil {
		return nil, err
	}

	return pa.transform(), nil
}

func (p *processor) Name() string {
	return processorName
}
