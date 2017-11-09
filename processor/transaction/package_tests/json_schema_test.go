package package_tests

import (
	"testing"

	"github.com/fatih/set"

	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	//only add attributes that should not be documented by the schema
	undocumented := set.New(
		"transactions.traces.stacktrace.vars.key",
		"transactions.context.request.headers.some-other-header",
		"transactions.context.request.headers.array",
		"transactions.context.request.env.SERVER_SOFTWARE",
		"transactions.context.request.env.GATEWAY_INTERFACE",
		"transactions.context.request.cookies.c1",
		"transactions.context.request.cookies.c2",
		"transactions.context.custom.my_key",
		"transactions.context.custom.some_other_value",
		"transactions.context.custom.and_objects",
		"transactions.context.custom.and_objects.foo",
		"transactions.context.tags.organization_uuid",
	)
	tests.TestPayloadAttributesInSchema(t, "transaction/payload.json", undocumented, "transactions/payload.json")
}
