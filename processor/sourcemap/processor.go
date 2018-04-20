package sourcemap

import (
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "sourcemap"
	smapDocType   = "sourcemap"
)

var (
	sourcemapUploadMetrics = monitoring.Default.NewRegistry("apm-server.processor.sourcemap", monitoring.PublishExpvar)
	validationCount        = monitoring.NewInt(sourcemapUploadMetrics, "validation.count")
	validationError        = monitoring.NewInt(sourcemapUploadMetrics, "validation.errors")
	decodingCount          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.count")
	decodingError          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.errors")
)

var schema = pr.CreateSchema(sourcemapSchema, processorName)

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

func (p *processor) Decode(raw map[string]interface{}) (pr.Payload, error) {
	decodingCount.Inc()

	decoder := utility.ManualDecoder{}
	pa := Payload{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	if decoder.Err != nil {
		decodingError.Inc()
		return nil, decoder.Err
	}
	return &pa, nil
}
