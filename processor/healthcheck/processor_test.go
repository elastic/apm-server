package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
)

func TestImplementProcessorInterface(t *testing.T) {
	p := NewProcessor(config.Config{})
	assert.NotNil(t, p)
	_, ok := p.(pr.Processor)
	assert.True(t, ok)
	assert.IsType(t, &processor{}, p)
}
