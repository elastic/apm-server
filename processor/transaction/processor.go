package transaction

import (
	"encoding/json"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/santhosh-tekuri/jsonschema"
)

func init() {
	pr.Registry.AddProcessor(BackendEndpoint, NewBackendProcessor())
	pr.Registry.AddProcessor(FrontendEndpoint, NewFrontendProcessor())
}

var (
	transactionMetrics = monitoring.Default.NewRegistry("apm-server.processor.transaction")
	transformations    = monitoring.NewInt(transactionMetrics, "transformations")
	validationCount    = monitoring.NewInt(transactionMetrics, "validation.count")
	validationError    = monitoring.NewInt(transactionMetrics, "validation.errors")
)

const (
	BackendEndpoint  = "/v1/transactions"
	FrontendEndpoint = "/v1/client-side/transactions"
	processorName    = "transaction"
)

func NewBackendProcessor() pr.Processor {
	return &processor{
		schema: pr.CreateSchema(transactionSchema, processorName),
		typ:    pr.Backend,
	}
}

func NewFrontendProcessor() pr.Processor {
	return &processor{
		schema: pr.CreateSchema(transactionSchema, processorName),
		typ:    pr.Frontend,
	}
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

func (p *processor) Type() int {
	return p.typ
}
