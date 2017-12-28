package package_tests

import (
	"testing"

	"github.com/fatih/set"

	er "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {
	//only add attributes that should not be documented by the schema
	undocumented := set.New(
		"errors.log.stacktrace.vars.key",
		"errors.exception.stacktrace.vars.key",
		"errors.exception.attributes.foo",
		"errors.context.custom.my_key",
		"errors.context.custom.some_other_value",
		"errors.context.custom.and_objects",
		"errors.context.custom.and_objects.foo",
		"errors.context.request.headers.some-other-header",
		"errors.context.request.headers.array",
		"errors.context.request.env.SERVER_SOFTWARE",
		"errors.context.request.env.GATEWAY_INTERFACE",
		"errors.context.request.cookies.c1",
		"errors.context.request.cookies.c2",
		"errors.context.tags.organization_uuid",
	)
	tests.TestPayloadAttributesInSchema(t, "error", undocumented, er.Schema())
}

func TestJsonSchemaKeywordLimitation(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	exceptions := set.New(
		"processor.event",
		"processor.name",
		"error.id",
		"error.log.level",
		"error.grouping_key",
		"transaction.id",
		"listening",
		"error id icon",
		"view errors",
	)
	tests.TestJsonSchemaKeywordLimitation(t, fieldsPaths, er.Schema(), exceptions)
}

func TestErrorPayloadSchema(t *testing.T) {
	testData := []tests.SchemaTestData{
		{File: "data/invalid/error_payload/no_service.json", Error: "missing properties: \"service\""},
		{File: "data/invalid/error_payload/no_errors.json", Error: "missing properties: \"errors\""},
	}
	tests.TestDataAgainstProcessor(t, er.NewProcessor(), testData)
}
