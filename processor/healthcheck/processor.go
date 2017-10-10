package healthcheck

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
)

func init() {
	pr.Registry.AddProcessor(Endpoint, NewProcessor())
}

const (
	Endpoint      = "/healthcheck"
	processorName = "healthcheck"
)

func NewProcessor() pr.Processor {
	return &processor{}
}

type processor struct{}

func (p *processor) Validate(buf []byte) error {
	return nil
}

func (p *processor) Transform(buf []byte) ([]beat.Event, error) {
	return nil, nil
}

func (p *processor) Name() string {
	return processorName
}

func (p *processor) Type() int {
	return pr.Nop
}
