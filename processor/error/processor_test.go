package error

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
)

func TestImplementProcessorInterface(t *testing.T) {
	constructors := []func() pr.Processor{NewFrontendProcessor, NewBackendProcessor}
	for _, constructor := range constructors {
		p := constructor()
		assert.NotNil(t, p)
		_, ok := p.(pr.Processor)
		assert.True(t, ok)
		assert.IsType(t, &processor{}, p)
	}
}

func TestAddProcessorToRegistryOnInit(t *testing.T) {
	p := pr.Registry.Processor(BackendEndpoint)
	assert.NotNil(t, p)
	assert.Equal(t, pr.Backend, p.Type())

	p2 := pr.Registry.Processor(FrontendEndpoint)
	assert.NotNil(t, p2)
	assert.Equal(t, pr.Frontend, p2.Type())
}
