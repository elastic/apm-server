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
		payloadAttrsNotInJsonSchema(tests.NewSet("spans.stacktrace.vars.key")),
		tests.NewSet(tests.Group("transactions.spans")),
	)
}

func TestDtAttrsPresenceInTransaction(t *testing.T) {
	reqKeys := requiredKeys(tests.NewSet(
		"spans.id",
		"spans.trace_id",
		"spans.parent_id",
		"spans.transaction_id",
		"spans.duration",
		"spans.name",
		"spans.start",
		"spans.type",
		"spans.stacktrace.filename",
		"spans.stacktrace.lineno",
		"transactions.trace_id",
		"transactions.transaction_id",
	))

	procSetup("dt").AttrsPresence(t, reqKeys,
		condRequiredKeys(map[string]tests.Condition{
			"transactions.spans.id": tests.Condition{Existence: map[string]interface{}{"transactions.spans.parent": float64(123)}},
		}))
	procSetup("dt").AttrsNotNullable(t, reqKeys)
}

func TestDtKeywordLimitationOnTransactionAttrs(t *testing.T) {
	procSetup("dt").KeywordLimitation(t, keywordExceptionKeys(nil),
		templateToSchemaMapping(map[string]string{"span": "spans"}))
}

func TestDtTracingPayloadDataForTransaction(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	procSetup("dt").DataValidation(t, schemaTestData(
		[]tests.SchemaTestData{
			{Key: "transactions.id",
				Valid:   []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"0123456789abCDeg", "abcdef01234567890"}}}},
			{Key: "transactions.trace_id",
				Valid: []interface{}{"0123456789abCdef0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `trace_id/pattern`,
					Values: val{"0123456789abcdef", "01234567890123456789abcdefGhabcdefZh", "01234567890123456789abcdefabcdeff"}}}},
			{Key: "transactions.parent_id",
				Valid:   []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `parent_id/pattern`, Values: val{"123", "85925e55-B43f-4340-a8e0-df1906ecbf7a"}}}},
			{Key: "spans", Valid: []interface{}{[]interface{}{}}},
			{Key: "spans.parent_id",
				Valid: []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `parent_id/pattern`,
					Values: val{"0123456789abcdez", "0123456789abcdef0", "85925e55-B43f-4340-a8e0-df1906ecbf7a"}}}},
			{Key: "spans.id",
				Valid: []interface{}{"0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `/id/pattern`,
					Values: val{"0123456789abcdez", "0123456789abcdef0", "85925e55-B43f-4340-a8e0-df1906ecbf7a"}}}},
			{Key: "spans.trace_id",
				Valid: []interface{}{"0123456789abCdef0123456789abcdef"},
				Invalid: []tests.Invalid{{Msg: `trace_id/pattern`,
					Values: val{"0123456789abcdef", "01234567890123456789abcdefGhabcdefZh", "01234567890123456789abcdefabcdeff"}}}},
			{Key: "spans.stacktrace.pre_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
			{Key: "spans.stacktrace.post_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
		}))
}
