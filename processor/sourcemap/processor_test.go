package sourcemap

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/common"
)

func TestImplementProcessorInterface(t *testing.T) {
	p := NewProcessor()
	assert.NotNil(t, p)
	_, ok := p.(pr.Processor)
	assert.True(t, ok)
	assert.IsType(t, &processor{}, p)
}

func TestValidate(t *testing.T) {
	p := NewProcessor()
	data, err := loader.LoadValidData("sourcemap")

	assert.NoError(t, err)
	err = p.Validate(data)
	assert.NoError(t, err)

	err = p.Validate(map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not in expected format"))

	err = p.Validate(map[string]interface{}{"sourcemap": ""})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "Error validating sourcemap"))

	delete(data, "bundle_filepath")
	err = p.Validate(data)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing properties"))
}

func TestTransform(t *testing.T) {
	data, err := loader.LoadValidData("sourcemap")
	assert.NoError(t, err)

	payload, err := NewProcessor().Decode(data)
	assert.NoError(t, err)
	rs := payload.Transform(config.Config{})
	assert.Len(t, rs, 1)
	event := rs[0]
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "js/bundle.js", getStr(output, "bundle_filepath"))
	assert.Equal(t, "service", getStr(output, "service.name"))
	assert.Equal(t, "1", getStr(output, "service.version"))
	assert.Equal(t, data["sourcemap"], getStr(output, "sourcemap"))

	payload, err = NewProcessor().Decode(nil)
	assert.Equal(t, errors.New("Error fetching field"), err)
}
