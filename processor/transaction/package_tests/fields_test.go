package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

func TestEsDocumentation(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	processorFn := transaction.NewProcessor
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, processorFn)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, processorFn)
}
