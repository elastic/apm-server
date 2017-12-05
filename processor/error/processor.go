package error

import (
	"encoding/json"
	"net/http"

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
)

var schema = pr.CreateSchema(errorSchema, processorName)

func NewProcessor(r *http.Request) pr.Processor {
	return &processor{schema: schema, req: r}
}

type processor struct {
	schema *jsonschema.Schema
	req    *http.Request
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

	return pa.transform(p.req), nil
}

func (p *processor) Name() string {
	return processorName
}
