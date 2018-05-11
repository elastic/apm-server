package transaction

import (
	"github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"

	sc "github.com/elastic/apm-server/processor/transaction/generated-schemas"
	"github.com/santhosh-tekuri/jsonschema"
)

var (
	transactionMetrics = monitoring.Default.NewRegistry("apm-server.processor.transaction", monitoring.PublishExpvar)
	decodingCount      = monitoring.NewInt(transactionMetrics, "decoding.count")
	decodingError      = monitoring.NewInt(transactionMetrics, "decoding.errors")
	validationCount    = monitoring.NewInt(transactionMetrics, "validation.count")
	validationError    = monitoring.NewInt(transactionMetrics, "validation.errors")
)

const (
	processorName      = "transaction"
	transactionDocType = "transaction"
	spanDocType        = "span"
)

var schema = pr.CreateSchema(sc.PayloadSchema, processorName)

func NewProcessor() pr.Processor {
	return &processor{schema: schema}
}

type processor struct {
	schema *jsonschema.Schema
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

func (p *processor) Decode(raw map[string]interface{}) (model.TransformableBatch, error) {
	decodingCount.Inc()
	txs, err := DecodePayload(raw)
	if err != nil {
		decodingError.Inc()
		return nil, err
	}

	transformables := []model.Transformable{}
	for _, tx := range txs {
		transformables = append(transformables, tx)
		for _, sp := range tx.Spans {
			transformables = append(transformables, sp)
		}
	}

	return transformables, nil
}
