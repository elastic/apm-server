package package_tests

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestTransactionProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessTransactionFull", Path: "tests/data/valid/transaction/payload.json"},
		{Name: "TestProcessTransactionMinimalPayload", Path: "tests/data/valid/transaction/minimal_payload.json"},
		{Name: "TestProcessTransactionMinimalTrace", Path: "tests/data/valid/transaction/minimal_trace.json"},
		{Name: "TestProcessTransactionEmpty", Path: "tests/data/valid/transaction/transaction_empty_values.json"},
		{Name: "TestProcessTransactionNull", Path: "tests/data/valid/transaction/transaction_null_values.json"},
	}
	tests.TestProcessRequests(t, transaction.NewProcessor, requestInfo)
}

// ensure all invalid documents fail the json schema validation already
func TestTransactionProcessorValidationFailed(t *testing.T) {
	invalidRequests := [][]string{
		{"app/invalid_name.json", "[#/properties/app/properties/name/pattern] does not match pattern"},
		{"app/agent_no_name.json", "missing properties: \"name\""},
		{"app/agent_no_version.json", "missing properties: \"version\""},
		{"app/framework_no_name.json", "missing properties: \"name\""},
		{"app/framework_no_version.json", "missing properties: \"version\""},
		{"app/lang_no_name.json", "missing properties: \"name\""},
		{"app/lang_no_version.json", "missing properties: \"version\""},
		{"app/runtime_no_name.json", "missing properties: \"name\""},
		{"app/runtime_no_version.json", "missing properties: \"version\""},
		{"app/no_name.json", "missing properties: \"name\""},
		{"app/no_agent.json", "missing properties: \"agent\""},
		{"no_app.json", "missing properties: \"app\""},
		{"invalid_tag_dot.json", "[#/properties/transactions/items/properties/context/properties/tags/additionalProperties]"},
		{"invalid_tag_asterisk.json", "[#/properties/transactions/items/properties/context/properties/tags/additionalProperties]"},
		{"empty_payload.json", "minimum 1 items allowed"},
		{"invalid_id.json", "[#/properties/transactions/items/properties/id/pattern] does not match pattern"},
		{"invalid_timestamp.json", "[#/properties/transactions/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp2.json", "[#/properties/transactions/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp3.json", "[#/properties/transactions/items/properties/timestamp/pattern] does not match pattern"},
		{"invalid_timestamp4.json", "[#/properties/transactions/items/properties/timestamp/pattern] does not match pattern"},
	}
	for _, dataRow := range invalidRequests {
		data, err := tests.LoadData(filepath.Join("tests/data/invalid/transaction/", dataRow[0]))
		assert.Nil(t, err)
		p := transaction.NewProcessor()
		err = p.Validate(bytes.NewReader(data))
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), dataRow[1]), fmt.Sprintf("'%v' not found in '%v'", dataRow[1], err.Error()))
	}
}
