package processor

import (
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/libbeat/logp"
)

type Register struct {
	processors map[string]ProcessorData
}

// Registry is a global registry that contains all processors
var Registry = newRegister()

// NewRegister creates a new register for the processors
func newRegister() *Register {
	return &Register{
		processors: map[string]ProcessorData{},
	}
}

type ProcessorData struct {
	Name      string
	Path      string
	Schema    *jsonschema.Schema
	Processor Processor
}

// AddProcessor adds a processor listening on the given path
func (r *Register) AddProcessor(path string, p Processor, schema string) {
	logp.Info("Processor for path registered: %s", path)

	procData := ProcessorData{
		Schema:    CreateSchema(schema, p.Name()),
		Processor: p,
		Path:      path,
		Name:      p.Name(),
	}

	r.processors[path] = procData
}

// GetProcessors returns a list of all currently registered processors
func (r *Register) Processors() map[string]ProcessorData {
	return r.processors
}

func (r *Register) Processor(name string) ProcessorData {
	return r.processors[name]
}
