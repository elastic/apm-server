package package_tests

import (
	"testing"

	"github.com/fatih/set"

	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/tests"
)

func TestEsDocumentation(t *testing.T) {
	fieldsPaths := []string{
		"./../_meta/fields.yml",
	}
	processorFn := sourcemap.NewProcessor
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, processorFn)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, processorFn, set.New())
}
