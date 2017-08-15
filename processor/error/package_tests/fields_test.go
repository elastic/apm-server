package package_tests

import (
	"testing"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

func TestFields(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, er.NewProcessor)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, er.NewProcessor)
}
