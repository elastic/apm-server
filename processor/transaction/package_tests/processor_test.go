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
		{Name: "TestProcessTransactionFull", Path: "data/valid/transaction/payload.json"},
		{Name: "TestProcessTransactionNullValues", Path: "data/valid/transaction/null_values.json"},
		{Name: "TestProcessSystemNull", Path: "data/valid/transaction/system_null.json"},
		{Name: "TestProcessTransactionMinimalPayload", Path: "data/valid/transaction/minimal_payload.json"},
		{Name: "TestProcessTransactionMinimalSpan", Path: "data/valid/transaction/minimal_span.json"},
		{Name: "TestProcessTransactionMinimalService", Path: "data/valid/transaction/minimal_service.json"},
		{Name: "TestProcessTransactionEmpty", Path: "data/valid/transaction/transaction_empty_values.json"},
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), requestInfo, map[string]string{})
}

// ensure invalid documents fail the json schema validation already
func TestTransactionProcessorValidationFailed(t *testing.T) {
	data, err := tests.LoadInvalidData("transaction")
	assert.Nil(t, err)
	p := transaction.NewProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
