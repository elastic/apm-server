package integration_tests

import (
	"testing"

	"github.com/fatih/set"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

func TestTemplate(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	undocumentedKeys := set.New("context.tags.organization_uuid")
	tests.TestEventAttrsDocumentedInEsSchemaTemplate(t, fieldsPaths, er.NewProcessor, undocumentedKeys)
	tests.TestEsDocumentedFieldsInEvent(t, fieldsPaths, er.NewProcessor, undocumentedKeys)
}
