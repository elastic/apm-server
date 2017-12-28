package stat

import (
	"github.com/mitchellh/mapstructure"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/utility"
)

var (
	transformations = monitoring.NewInt(statMetrics, "transformations")
	validationCount = monitoring.NewInt(statMetrics, "validation.count")
	validationError = monitoring.NewInt(statMetrics, "validation.errors")
)

const (
	processorName = "stat"
)

var schema = pr.CreateSchema(statSchema, processorName)

func NewProcessor(config *pr.Config) pr.Processor {
	return &processor{schema: schema, config: config}
}

type processor struct {
	schema *jsonschema.Schema
	config *pr.Config
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

	return pa.transform(p.config), nil
}

func (p *processor) Name() string {
	return processorName
}
