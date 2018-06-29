package metric

import (
	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/processor/metric/generated/schema"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "metric"
	docType       = "metric"
)

var (
	metricMetrics   = monitoring.Default.NewRegistry("apm-server.processor.metric", monitoring.PublishExpvar)
	validationCount = monitoring.NewInt(metricMetrics, "validation.count")
	validationError = monitoring.NewInt(metricMetrics, "validation.error")
	decodingCount   = monitoring.NewInt(metricMetrics, "decoding.count")
	decodingError   = monitoring.NewInt(metricMetrics, "decoding.errors")
)

type processor struct {
	schema *jsonschema.Schema
}

var loadedSchema = pr.CreateSchema(schema.PayloadSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: loadedSchema}
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	err := pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Decode(raw map[string]interface{}) (pr.Payload, error) {
	decodingCount.Inc()
	pa, err := DecodePayload(raw)
	if err != nil {
		decodingError.Inc()
		return nil, err
	}
	return pa, nil
}
