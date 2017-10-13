package package_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessErrorMininmalPayloadException", Path: "tests/data/valid/error/minimal_payload_exception.json"},
		{Name: "TestProcessErrorMininmalPayloadLog", Path: "tests/data/valid/error/minimal_payload_log.json"},
		{Name: "TestProcessErrorFull", Path: "tests/data/valid/error/payload.json"},
	}
	tests.TestProcessRequests(t, er.NewProcessor(), requestInfo)
}

// ensure invalid documents fail the json schema validation already
func TestProcessorFailedValidation(t *testing.T) {
	data, err := tests.LoadInvalidData("error")
	assert.Nil(t, err)
	err = er.NewProcessor().Validate(data)
	assert.NotNil(t, err)
}
