package package_tests

import (
	"testing"

	"github.com/fatih/set"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

func TestEsDocumentation(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	undocumentedKeys := set.New("context.tags.organization_uuid", "context.sql")
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, transaction.NewProcessor, undocumentedKeys)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, transaction.NewProcessor, undocumentedKeys)
}
