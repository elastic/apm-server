package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
	"github.com/stretchr/testify/assert"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestSourcemapProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessSourcemapFull", Path: "data/valid/sourcemap/payload.json"},
		{Name: "TestProcessSourcemapMinimalPayload", Path: "data/valid/sourcemap/minimal_payload.json"},
	}
	tests.TestProcessRequests(t, sourcemap.NewProcessor(), requestInfo, map[string]string{"@timestamp": "***IGNORED***"})
}

// ensure invalid documents fail the json schema validation already
func TestSourcemapProcessorValidationFailed(t *testing.T) {
	data, err := tests.LoadInvalidData("sourcemap")
	assert.Nil(t, err)
	p := sourcemap.NewProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
