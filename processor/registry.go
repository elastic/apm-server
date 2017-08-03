package processor

import (
	"github.com/elastic/beats/libbeat/logp"
)

type Register struct {
	processors map[string]Processor
}

// Registry is a global registry that contains all processors
var Registry = newRegister()

// NewRegister creates a new register for the processors
func newRegister() *Register {
	return &Register{
		processors: map[string]Processor{},
	}
}

// AddProcessor adds a processor listening on the given path
func (r *Register) AddProcessor(path string, p Processor) {
	logp.Info("Processor for path registered: %s", path)
	r.processors[path] = p
}

// GetProcessors returns a list of all currently registered processors
func (r *Register) GetProcessors() map[string]Processor {
	return r.processors
}

func (r *Register) GetProcessor(name string) Processor {
	return r.processors[name]
}
