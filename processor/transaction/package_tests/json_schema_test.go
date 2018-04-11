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
		"transactions.context.request.body.str",
		"transactions.context.request.body.additional",
		"transactions.context.request.body.additional.foo",
		"transactions.context.request.body.additional.bar",
		"transactions.context.request.body.additional.req",
		"transactions.context.request.cookies.c1",
		"transactions.context.request.cookies.c2",
		"transactions.context.custom",
		"transactions.context.custom.my_key",
		"transactions.context.custom.some_other_value",
		"transactions.context.custom.and_objects",
		"transactions.context.custom.and_objects.foo",
		"transactions.context.tags",
		"transactions.context.tags.organization_uuid",
		"transactions.marks.navigationTiming",
		"transactions.marks.navigationTiming.appBeforeBootstrap",
		"transactions.marks.navigationTiming.navigationStart",
		"transactions.marks.performance",
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

func TestTransactionPayloadSchema(t *testing.T) {
	testData := []tests.SchemaTestData{
		{File: "data/invalid/transaction_payload/no_service.json", Error: "missing properties: \"service\""},
		{File: "data/invalid/transaction_payload/no_transactions.json", Error: "minimum 1 items allowed"},
	}
	tests.TestDataAgainstProcessor(t, transaction.NewProcessor(), testData)
}
