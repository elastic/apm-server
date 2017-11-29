package sourcemap

import (
	"encoding/json"
	"net/http"

	"github.com/santhosh-tekuri/jsonschema"

	parser "github.com/go-sourcemap/sourcemap"

	pr "github.com/elastic/apm-server/processor"
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

type processor struct {
	schema *jsonschema.Schema
}

func NewProcessor(*http.Request) pr.Processor {
	return &processor{schema}
}

func (p *processor) Validate(buf []byte) error {
	validationCount.Inc()

	var pa payload
	err := json.Unmarshal(buf, &pa)
	if err != nil {
		return err
	}

	sourcemapBytes, err := json.Marshal(pa.Sourcemap)
	if err != nil {
		return err
	}

	_, err = parser.Parse("", sourcemapBytes)
	if err != nil {
		return err
	}

	err = pr.Validate(buf, p.schema)
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
