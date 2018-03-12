package sourcemap

import (
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "sourcemap"
	smapDocType   = "sourcemap"
)

var (
	sourcemapUploadMetrics = monitoring.Default.NewRegistry("apm-server.processor.sourcemap")
	transformations        = monitoring.NewInt(sourcemapUploadMetrics, "transformations")
	validationCount        = monitoring.NewInt(sourcemapUploadMetrics, "validation.count")
	validationError        = monitoring.NewInt(sourcemapUploadMetrics, "validation.errors")
)

var schema = pr.CreateSchema(sourcemapSchema, processorName)

func NewProcessor(config *pr.Config) pr.Processor {
	return &processor{schema: schema, config: config}
}

type processor struct {
	schema *jsonschema.Schema
	config *pr.Config
}

func (p *processor) Validate(input pr.Intake) error {
	validationCount.Inc()

	_, err := parser.Parse("", input.Data)
	if err != nil {
		return errors.New(fmt.Sprintf("Error validating sourcemap: %v", err))
	}

	data := map[string]interface{}{
		"service_name":    input.ServiceName,
		"service_version": input.ServiceVersion,
		"bundle_filepath": input.BundleFilepath,
	}
	if err = p.schema.ValidateInterface(data); err != nil {
		validationError.Inc()
		return fmt.Errorf("Problem validating JSON document against schema: %v", err)
	}
	return nil
}

func (p *processor) Transform(input pr.Intake) ([]beat.Event, error) {
	transformations.Inc()

	pa := payload{
		Sourcemap:      string(input.Data),
		ServiceName:    input.ServiceName,
		ServiceVersion: input.ServiceVersion,
		BundleFilepath: input.BundleFilepath,
	}
	return pa.transform(p.config), nil
}

func (p *processor) Name() string {
	return processorName
}
