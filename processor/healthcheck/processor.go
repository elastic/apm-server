package healthcheck

import (
	"net/http"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/beat"
)

const (
	processorName = "healthcheck"
)

func NewProcessor(*http.Request) pr.Processor {
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
