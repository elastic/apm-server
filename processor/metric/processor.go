package metric

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	processorName = "metric"
	docType       = "metric"
)

var (
	sourcemapUploadMetrics = monitoring.Default.NewRegistry("apm-server.processor.metric", monitoring.PublishExpvar)
	validationCount        = monitoring.NewInt(sourcemapUploadMetrics, "validation.count")
	decodingCount          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.count")
	decodingError          = monitoring.NewInt(sourcemapUploadMetrics, "decoding.errors")
)

type processor struct{}

func NewProcessor() pr.Processor {
	return &processor{}
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	return nil
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
