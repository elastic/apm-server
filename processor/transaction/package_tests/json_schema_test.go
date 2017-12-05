package package_tests

import (
	"testing"

	"github.com/fatih/set"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	//only add attributes that should not be documented by the schema
	undocumented := set.New(
		"transactions.spans.stacktrace.vars.key",
		"transactions.context.request.headers.some-other-header",
		"transactions.context.request.headers.array",
		"transactions.context.request.env.SERVER_SOFTWARE",
		"transactions.context.request.env.GATEWAY_INTERFACE",
		"transactions.context.request.body",
		"transactions.context.request.cookies.c1",
		"transactions.context.request.cookies.c2",
		"transactions.context.custom",
		"transactions.context.custom.my_key",
		"transactions.context.custom.some_other_value",
		"transactions.context.custom.and_objects",
		"transactions.context.custom.and_objects.foo",
		"transactions.context.tags",
		"transactions.context.tags.organization_uuid",
	)
	tests.TestPayloadAttributesInSchema(t, "transaction", undocumented, transaction.Schema())
}

func TestJsonSchemaKeywordLimitation(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	exceptions := set.New("processor.event", "processor.name", "context.service.name", "transaction.id", "listening")
	tests.TestJsonSchemaKeywordLimitation(t, fieldsPaths, transaction.Schema(), exceptions)
}
