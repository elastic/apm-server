package package_tests

import (
	"testing"

	"github.com/fatih/set"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

func TestFields(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	undocumentedKeys := set.New("context.tags.organization_uuid")
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, er.NewProcessor, undocumentedKeys)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, er.NewProcessor, undocumentedKeys)
}
