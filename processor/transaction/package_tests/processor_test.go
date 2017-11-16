package package_tests

import (
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestTransactionProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessTransactionFull", Path: "tests/data/valid/transaction/payload.json"},
		{Name: "TestProcessTransactionNullValues", Path: "tests/data/valid/transaction/null_values.json"},
		{Name: "TestProcessSystemNull", Path: "tests/data/valid/transaction/system_null.json"},
		{Name: "TestProcessTransactionMinimalPayload", Path: "tests/data/valid/transaction/minimal_payload.json"},
		{Name: "TestProcessTransactionMinimalTrace", Path: "tests/data/valid/transaction/minimal_trace.json"},
		{Name: "TestProcessTransactionMinimalApp", Path: "tests/data/valid/transaction/minimal_app.json"},
		{Name: "TestProcessTransactionEmpty", Path: "tests/data/valid/transaction/transaction_empty_values.json"},
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), requestInfo, *set.New())
}

// ensure invalid documents fail the json schema validation already
func TestTransactionProcessorValidationFailed(t *testing.T) {
	data, err := tests.LoadInvalidData("transaction")
	assert.Nil(t, err)
	p := transaction.NewProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
