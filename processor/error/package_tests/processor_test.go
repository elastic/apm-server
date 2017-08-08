package package_tests

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"fmt"

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
	tests.TestProcessRequests(t, er.NewProcessor, requestInfo)
}

// ensure all invalid documents fail the json schema validation already
func TestProcessorFailedValidation(t *testing.T) {
	invalidRequests := [][]string{
		{"invalid_app_name.json", "[#/properties/app/properties/name/pattern] does not match pattern"},
		{"invalid_id.json", "[#/properties/errors/items/properties/id/pattern] does not match pattern"},
		{"invalid_timestamp.json", "[#/properties/errors/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp2.json", "[#/properties/errors/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp3.json", "[#/properties/errors/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp4.json", "[#/properties/errors/items/properties/timestamp/pattern] does not match pattern"},
		{"no_log_message.json", "missing properties: \"message\""},
		{"no_exception_message.json", "missing properties: \"message\""},
		{"no_log_or_exception.json", "missing properties: \"exception\""},
		{"no_log_or_exception.json", "missing properties: \"log\""},
		{"no_timestamp.json", "missing properties: \"timestamp\""},
		{"no_app.json", "missing properties: \"app\""},
		{"no_http_url.json", "missing properties: \"url\""},
		{"no_http_method.json", "missing properties: \"method\""},
		{"no_stacktrace_filename.json", "missing properties: \"filename\""},
		{"no_stacktrace_lineno.json", "missing properties: \"lineno\""},
		{"non_string_tags.json", "expected string, but got object"},
	}
	for _, dataRow := range invalidRequests {
		data, err := tests.LoadData(filepath.Join("tests/data/invalid/error/", dataRow[0]))
		assert.Nil(t, err)
		err = er.NewProcessor().Validate(bytes.NewReader(data))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), dataRow[1]), fmt.Sprintf("'%v' not found in '%v'", dataRow[1], err.Error()))
	}
}
