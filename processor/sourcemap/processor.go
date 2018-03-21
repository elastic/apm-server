package sourcemap

import (
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
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

func NewProcessor(config pr.Config) pr.Processor {
	return &processor{schema: schema, config: config}
}

func (p *processor) Name() string {
	return processorName
}

type processor struct {
	schema *jsonschema.Schema
	config pr.Config
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()

	smap, ok := raw["sourcemap"].(string)
	if !ok {
		return errors.New("Sourcemap not in expected format.")
	}

	_, err := parser.Parse("", []byte(smap))
	if err != nil {
		return errors.New(fmt.Sprintf("Error validating sourcemap: %v", err))
	}

	err = pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(raw map[string]interface{}) ([]beat.Event, error) {

	transformations.Inc()

	decoder := utility.ManualDecoder{}
	pa := payload{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}

	if decoder.Err != nil {
		return nil, decoder.Err
	}
	return pa.transform(p.config), nil
}
