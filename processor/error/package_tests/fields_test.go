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
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, er.NewBackendProcessor)

	notInEvent := set.New(
		"context.db.instance",
		"context.db.statement",
		"context.db.user",
		"context.db.type",
		"context.db",
		"listening",
		"error id icon",
		"view errors",
	)
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, er.NewBackendProcessor, notInEvent)
}
