package healthcheck

import (
	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
)

const (
	processorName = "healthcheck"
)

func NewProcessor() pr.Processor {
	return &processor{}
}

type processor struct{}

func (p *processor) Validate(_ map[string]interface{}) error {
	return nil
}

func (p *processor) Decode(_ config.Config, _ map[string]interface{}) (pr.Payload, error) {
	return nil, nil
}

func (p *processor) Name() string {
	return processorName
}
