package integration_tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {
	var payload interface{}
	tests.UnmarshalValidData("transaction", &payload)
	jsonKeys := []string{}
	tests.FlattenJsonKeys(payload, "", &jsonKeys)

	schema, err := tests.GetSchemaProperties(strings.NewReader(transaction.Schema()))
	assert.Nil(t, err)
	schemaKeys := []string{}
	tests.FlattenSchemaProperties(schema, "", &schemaKeys)

	//only add attributes that should not be documented by the schema
	undocumented := []string{
		"transactions.traces.stacktrace.vars.key",
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
	}
	jsonKeysDoc, _ := tests.ArrayDiff(jsonKeys, undocumented)
	missing, _ := tests.ArrayDiff(jsonKeysDoc, schemaKeys)
	if len(missing) > 0 {
		msg := fmt.Sprintf("Json Transaction Payload fields missing in Schema %v", missing)
		assert.Fail(t, msg)
	}

	missing, _ = tests.ArrayDiff(schemaKeys, jsonKeys)
	if len(missing) > 0 {
		msg := fmt.Sprintf("Json Transaction schema fields missing in Payload %v", missing)
		assert.Fail(t, msg)
	}
}
