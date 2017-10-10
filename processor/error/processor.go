package error

import (
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
)

func init() {
	pr.Registry.AddProcessor(BackendEndpoint, NewBackendProcessor())
	pr.Registry.AddProcessor(FrontendEndpoint, NewFrontendProcessor())
}

var (
	errorMetrics    = monitoring.Default.NewRegistry("apm-server.processor.error")
	validationCount = monitoring.NewInt(errorMetrics, "validation.count")
	validationError = monitoring.NewInt(errorMetrics, "validation.errors")
	transformations = monitoring.NewInt(errorMetrics, "transformations")
)

const (
	BackendEndpoint  = "/v1/errors"
	FrontendEndpoint = "/v1/client-side/errors"
	processorName    = "error"
)

func NewBackendProcessor() pr.Processor {
	schema := pr.CreateSchema(errorSchema, processorName)
	return &processor{schema, pr.Backend}
}

func NewFrontendProcessor() pr.Processor {
	schema := pr.CreateSchema(errorSchema, processorName)
	return &processor{schema, pr.Frontend}
}

type processor struct {
	schema *jsonschema.Schema
	typ    int
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
	transformations.Inc()
	var pa payload
	err := json.Unmarshal(buf, &pa)
	if err != nil {
		return nil, err
	}

	return pa.transform(), nil
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Type() int {
	return p.typ
}
