package sourcemap

import (
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	"github.com/mitchellh/mapstructure"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "sourcemap"
)

var (
	sourcemapUploadMetrics = monitoring.Default.NewRegistry("apm-server.processor.sourcemap")
	transformations        = monitoring.NewInt(sourcemapUploadMetrics, "transformations")
	validationCount        = monitoring.NewInt(sourcemapUploadMetrics, "validation.count")
	validationError        = monitoring.NewInt(sourcemapUploadMetrics, "validation.errors")
)

var schema = pr.CreateSchema(sourcemapSchema, processorName)

func NewProcessor(conf *pr.Config) pr.Processor {
	var smapAccessor utility.SmapAccessor
	if conf != nil {
		smapAccessor = conf.SmapAccessor
	}
	return &processor{schema: schema, smapAccessor: smapAccessor}
}

type processor struct {
	schema       *jsonschema.Schema
	smapAccessor utility.SmapAccessor
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()

	bytes, err := json.Marshal(raw["sourcemap"])
	if err != nil {
		return err
	}

	_, err = parser.Parse("", bytes)
	if err != nil {
		return err
	}

	err = pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(raw interface{}) ([]beat.Event, error) {
	var pa payload
	transformations.Inc()

	err := mapstructure.Decode(raw, &pa)
	if err != nil {
		return nil, err
	}

	return pa.transform(p.smapAccessor), nil
}

func (p *processor) Name() string {
	return processorName
}
