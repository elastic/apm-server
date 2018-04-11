package package_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestSourcemapProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessSourcemapFull", Path: "data/valid/sourcemap/payload.json"},
		{Name: "TestProcessSourcemapMinimalPayload", Path: "data/valid/sourcemap/minimal_payload.json"},
	}
	tests.TestProcessRequests(t, sourcemap.NewProcessor(), config.Config{}, requestInfo, map[string]string{"@timestamp": "***IGNORED***"})
}

// ensure invalid documents fail the json schema validation already
func TestSourcemapProcessorValidationFailed(t *testing.T) {
	data, err := loader.LoadInvalidData("sourcemap")
	assert.Nil(t, err)
	p := sourcemap.NewProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
