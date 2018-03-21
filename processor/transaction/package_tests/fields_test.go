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
	processorFn := transaction.NewProcessor
	tests.TestEventAttrsDocumentedInFields(t, fieldsPaths, processorFn)
	// IP and user agent are generated in the server, so they do not come from a payload
	tests.TestDocumentedFieldsInEvent(t, fieldsPaths, processorFn,
		set.New("listening", "view spans", "context.user.user-agent", "context.user.ip", "context.system.ip"))
}
