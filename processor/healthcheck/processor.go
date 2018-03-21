package healthcheck

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
)

const (
	processorName = "healthcheck"
)

func NewProcessor(_ *pr.Config) pr.Processor {
	return &processor{}
}

type processor struct{}

func (p *processor) Validate(_ map[string]interface{}) error {
	return nil
}

func (p *processor) Transform(_ map[string]interface{}) ([]beat.Event, error) {
	return nil, nil
}

func (p *processor) Name() string {
	return processorName
}
