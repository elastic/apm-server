package integration_tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {
	var payload interface{}
	tests.UnmarshalValidData("error", &payload)
	jsonKeys := []string{}
	tests.FlattenJsonKeys(payload, "", &jsonKeys)

	schema, err := tests.GetSchemaProperties(strings.NewReader(error.Schema()))
	assert.Nil(t, err)
	schemaKeys := []string{}
	tests.FlattenSchemaProperties(schema, "", &schemaKeys)

	//only add attributes that should not be documented by the schema
	undocumented := []string{
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
	}
	jsonKeysDoc, _ := tests.ArrayDiff(jsonKeys, undocumented)
	missing, _ := tests.ArrayDiff(jsonKeysDoc, schemaKeys)
	if len(missing) > 0 {
		msg := fmt.Sprintf("Json Error Payload fields missing in Schema %v", missing)
		assert.Fail(t, msg)
	}
	missing, _ = tests.ArrayDiff(schemaKeys, jsonKeys)
	if len(missing) > 0 {
		msg := fmt.Sprintf("Json Error Schema fields missing in Payload %v", missing)
		assert.Fail(t, msg)
	}
}
