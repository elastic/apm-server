package package_tests

import (
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
	tests.TestProcessRequests(t, transaction.NewBackendProcessor(), requestInfo)
}

// ensure invalid documents fail the json schema validation already
func TestTransactionProcessorValidationFailed(t *testing.T) {
	data, err := tests.LoadInvalidData("transaction")
	assert.Nil(t, err)
	p := transaction.NewBackendProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
