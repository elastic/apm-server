package package_tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
	er "github.com/elastic/apm-server/processor/stat"
	"github.com/elastic/apm-server/tests"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestProcessorBackendOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessStatFull", Path: "data/valid/stat/payload.json"},
		{Name: "TestProcessStatNoItems", Path: "data/valid/stat/no_items.json"},
	}
	conf := processor.Config{ExcludeFromGrouping: nil}
	tests.TestProcessRequests(t, er.NewProcessor(&conf), requestInfo, map[string]string{})
}

// ensure invalid documents fail the json schema validation already
func TestProcessorFailedValidation(t *testing.T) {
	data, err := tests.LoadInvalidData("stat")
	assert.Nil(t, err)
	err = er.NewProcessor(nil).Validate(data)
	assert.NotNil(t, err)
}
