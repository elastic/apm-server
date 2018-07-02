package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func TestDtPayloadMatchFields(t *testing.T) {
	procSetup("dt").PayloadAttrsMatchFields(t,
		payloadAttrsNotInFields(nil), fieldsNotInPayloadAttrs(nil),
	)
}

func TestDtPayloadMatchJsonSchema(t *testing.T) {
	procSetup("dt").PayloadAttrsMatchJsonSchema(t,
		payloadAttrsNotInJsonSchema(nil),
		tests.NewSet(tests.Group("errors.transaction")),
	)
}

func TestDtAttrsPresenceInError(t *testing.T) {
	procSetup("dt").AttrsPresence(t,
		requiredKeys(tests.NewSet("errors.transaction_id", "errors.parent_id")),
		condRequiredKeys(nil))

	procSetup("dt").AttrsNotNullable(t, requiredKeys(tests.NewSet("errors.trace_id", "errors.transaction_id", "errors.parent_id")))
}

func TestDtKeywordLimitationOnErrorAttributes(t *testing.T) {
	procSetup("dt").KeywordLimitation(t, keywordExceptionKeys(nil), templateToSchemaMapping(nil))
}

func TestDtPayloadDataForError(t *testing.T) {
	//// add test data for testing
	//// * specific edge cases
	//// * multiple allowed dataypes
	//// * regex pattern, time formats
	//// * length restrictions, other than keyword length restrictions
	procSetup("dt").DataValidation(t, schemaTestData(
		[]tests.SchemaTestData{
			{Key: "errors.trace_id",
				Valid: []interface{}{"0123456789abcDEF0123456789ABCdef"},
				Invalid: []tests.Invalid{{Msg: `trace_id/pattern`,
					Values: val{"85925e55-B43f-4340-a8e0-df1906ecbf7a", "0123456789A", "0123456789abcdef0123456789ABCDEG"}}}},
			{Key: "errors.transaction_id",
				Valid: []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `transaction_id/pattern`,
					Values: val{"0123456789abcdez", "85925e55-b43f-4340-a8e0-df1906ecbf7a"}}}},
			{Key: "errors.parent_id",
				Valid: []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `parent_id/pattern`,
					Values: val{"0123456789abcdeg", "85925e55-b43f-4340-a8e0-df1906ecbf7a"}}}},
		}))
}
