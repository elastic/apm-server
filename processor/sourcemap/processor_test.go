package sourcemap

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/common"
)

func TestImplementProcessorInterface(t *testing.T) {
	p := NewProcessor(nil)
	assert.NotNil(t, p)
	_, ok := p.(pr.Processor)
	assert.True(t, ok)
	assert.IsType(t, &processor{}, p)
}

func TestValidate(t *testing.T) {
	p := NewProcessor(nil)
	data, err := loader.LoadValidData("sourcemap")
	assert.NoError(t, err)
	err = p.Validate(data)
	assert.NoError(t, err)

	err = p.Validate(pr.Intake{Data: []byte{}})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Error validating sourcemap"))

	err = p.Validate(pr.Intake{Data: data.Data})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Problem validating"))
}

func TestTransform(t *testing.T) {
	data, err := loader.LoadValidData("sourcemap")
	assert.NoError(t, err)

	rs, err := NewProcessor(nil).Transform(data)
	assert.NoError(t, err)

	assert.Len(t, rs, 1)
	event := rs[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)

	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "js/bundle.js", getStr(output, "bundle_filepath"))
	assert.Equal(t, "service", getStr(output, "service.name"))
	assert.Equal(t, "1", getStr(output, "service.version"))
	assert.Equal(t, string(data.Data), getStr(output, "sourcemap"))
}
