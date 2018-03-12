package error

import (
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
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

func NewProcessor(config *pr.Config) pr.Processor {
	return &processor{schema: schema, config: config}
}

type processor struct {
	schema *jsonschema.Schema
	config *pr.Config
}

func (p *processor) Validate(input pr.Intake) error {
	validationCount.Inc()
	err := pr.Validate(input.Data, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(input pr.Intake) ([]beat.Event, error) {
	transformations.Inc()

	var pa payload
	err := json.Unmarshal(input.Data, &pa)
	if err != nil {
		return nil, err
	}

	pa.User = pa.User.Enrich(input)
	pa.System = pa.System.Enrich(input)

	return pa.transform(p.config), nil
}

func (p *processor) Name() string {
	return processorName
}
