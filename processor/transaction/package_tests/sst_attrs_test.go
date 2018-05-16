package package_tests

import (
	"testing"

	"github.com/elastic/apm-server/tests"
)

func TestPayloadMatchFields(t *testing.T) {
	procSetup("sst").PayloadAttrsMatchFields(t, payloadAttrsNotInFields(nil),
		fieldsNotInPayloadAttrs(tests.NewSet("span.trace_id", "span.parent_id",
			"transaction.parent_id", "transaction.trace_id")))
}

func TestPayloadMatchJsonSchema(t *testing.T) {
	procSetup("sst").PayloadAttrsMatchJsonSchema(t,
		payloadAttrsNotInJsonSchema(tests.NewSet("transactions.spans.stacktrace.vars.key")),
		tests.NewSet(tests.Group("spans"), "transactions.parent_id", "transactions.trace_id"))
}

func TestAttrsPresenceInTransaction(t *testing.T) {
	procSetup("sst").AttrsPresence(t,
		requiredKeys(tests.NewSet(
			"transactions.spans.duration",
			"transactions.spans.name",
			"transactions.spans.start",
			"transactions.spans.type",
			"transactions.spans.stacktrace.filename",
			"transactions.spans.stacktrace.lineno",
		)),
		condRequiredKeys(map[string]tests.Condition{
			"transactions.spans.id": tests.Condition{Existence: map[string]interface{}{"transactions.spans.parent": float64(123)}},
		}))
}

func TestKeywordLimitationOnTransactionAttrs(t *testing.T) {
	procSetup("sst").KeywordLimitation(t, keywordExceptionKeys(nil),
		templateToSchemaMapping(map[string]string{"span": "transactions.spans"}))
}

func TestPayloadDataForTransaction(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	procSetup("sst").DataValidation(t, schemaTestData(
		[]tests.SchemaTestData{
			{Key: "transactions.id",
				Valid:   []interface{}{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
				Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7"}}}},
			{Key: "transactions.spans", Valid: []interface{}{[]interface{}{}}},
			{Key: "transactions.spans.id",
				Valid:   []interface{}{float64(123)},
				Invalid: []tests.Invalid{{Msg: `missing properties: "trace_id"`, Values: val{"1234567890abcdef"}}}},
			{Key: "transactions.spans.stacktrace.pre_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
			{Key: "transactions.spans.stacktrace.post_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
		}))
}
