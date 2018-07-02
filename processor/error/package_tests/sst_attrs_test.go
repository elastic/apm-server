package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func TestPayloadAttrsMatchFields(t *testing.T) {
	procSetup("sst").PayloadAttrsMatchFields(t,
		payloadAttrsNotInFields(nil),
		fieldsNotInPayloadAttrs(tests.NewSet("error.trace_id", "error.parent_id")),
	)
}

func TestPayloadAttrsMatchJsonSchema(t *testing.T) {
	procSetup("sst").PayloadAttrsMatchJsonSchema(t,
		payloadAttrsNotInJsonSchema(nil),
		tests.NewSet("errors.transaction_id", "errors.trace_id", "errors.parent_id"),
	)
}

func TestAttrsPresenceInError(t *testing.T) {
	procSetup("sst").AttrsPresence(t, requiredKeys(nil), condRequiredKeys(nil))
	procSetup("sst").AttrsNotNullable(t, requiredKeys(nil))
}

func TestKeywordLimitationOnErrorAttributes(t *testing.T) {
	procSetup("sst").KeywordLimitation(t, keywordExceptionKeys(nil), templateToSchemaMapping(nil))
}

func TestPayloadDataForError(t *testing.T) {
	//// add test data for testing
	//// * specific edge cases
	//// * multiple allowed dataypes
	//// * regex pattern, time formats
	//// * length restrictions, other than keyword length restrictions
	procSetup("sst").DataValidation(t, schemaTestData(
		[]tests.SchemaTestData{
			{Key: "errors.id", Valid: val{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
				Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7", "0123456789abcdef"}}}},
			{Key: "errors.transaction.id",
				Valid: val{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
				Invalid: []tests.Invalid{{Msg: `transaction/properties/id/pattern`, Values: val{"123",
					"z5925e55-b43f-4340-a8e0-df1906ecbf7a", "z5925e55-b43f-4340-a8e0-df1906ecbf7ia"}}}},
		}))
}
