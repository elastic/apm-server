package package_tests

import (
	tr "github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/processor/transaction/generated/schema"
	"github.com/elastic/apm-server/tests"
)

type obj = map[string]interface{}
type val = []interface{}

func procSetup(tracingType string) *tests.ProcessorSetup {
	var payloadPath string
	if tracingType == "dt" {
		payloadPath = "../testdata/transaction/dt_payload.json"
	} else {
		payloadPath = "../testdata/transaction/payload.json"
	}
	return &tests.ProcessorSetup{
		Proc:            tr.NewProcessor(),
		FullPayloadPath: payloadPath,
		TemplatePaths: []string{"../_meta/fields.yml",
			"../../../_meta/fields.common.yml"},
		Schema: schema.PayloadSchema,
	}
}

func payloadAttrsNotInFields(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet("span.stacktrace", tests.Group("transaction.marks."), tests.Group("context.db"),
		"context.http", "context.http.url"))
}

func fieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"listening", "view spans", "context.user.user-agent",
		"context.user.ip", "context.system.ip",
		"transaction.hex_id", "span.hex_id"))
}

func payloadAttrsNotInJsonSchema(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"transactions.context.request.headers.some-other-header",
		"transactions.context.request.headers.array",
		tests.Group("transactions.context.request.env."),
		tests.Group("transactions.context.request.body"),
		tests.Group("transactions.context.request.cookies"),
		tests.Group("transactions.context.custom"),
		tests.Group("transactions.context.tags"),
		tests.Group("transactions.marks"),
	))
}

func requiredKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"transactions",
		"transactions.id",
		"transactions.duration",
		"transactions.type",
		"transactions.context.request.method",
		"transactions.context.request.url",
	))
}

func condRequiredKeys(c map[string]tests.Condition) map[string]tests.Condition {
	return c
}

func keywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"processor.event", "processor.name", "listening",
		"transaction.id", "transaction.parent_id", "transaction.trace_id",
		"transaction.hex_id", "transaction.marks", "context.tags",
		"span.hex_id", "span.trace_id", "span.parent_id"))
}

func templateToSchemaMapping(mapping map[string]string) map[string]string {
	base := map[string]string{
		"context.system.":  "system.",
		"context.process.": "process.",
		"context.service.": "service.",
		"context.request.": "transactions.context.request.",
		"context.user.":    "transactions.context.user.",
		"transaction.":     "transactions.",
	}
	for k, v := range mapping {
		base[k] = v
	}
	return base
}

func schemaTestData(td []tests.SchemaTestData) []tests.SchemaTestData {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions
	if td == nil {
		td = []tests.SchemaTestData{}
	}
	return append(td, []tests.SchemaTestData{
		{Key: "service.name",
			Valid:   val{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `service/properties/name`, Values: val{tests.Str1024Special, tests.Str1025}}}},
		{Key: "process.argv",
			Valid:   []interface{}{[]interface{}{}, []interface{}{"a"}},
			Invalid: []tests.Invalid{{Msg: `argv/type`, Values: val{123, tests.Str1024}}}},
		{Key: "transactions", Invalid: []tests.Invalid{
			{Msg: `transactions/type`, Values: val{false}},
			{Msg: `transactions/minitems`, Values: val{[]interface{}{}}}}},
		{Key: "transactions.id",
			Valid: []interface{}{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
			Invalid: []tests.Invalid{
				{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7"}}}},
		{Key: "transactions.duration",
			Valid:   []interface{}{12.4},
			Invalid: []tests.Invalid{{Msg: `duration/type`, Values: val{"123"}}}},
		{Key: "transactions.timestamp",
			Valid: val{"2017-05-30T18:53:42.281Z"},
			Invalid: []tests.Invalid{
				{Msg: `timestamp/format`, Values: val{"2017-05-30T18:53Z", "2017-05-30T18:53:27.Z", "2017-05-30T18:53:27a123Z"}},
				{Msg: `timestamp/pattern`, Values: val{"2017-05-30T18:53:27.000+00:20", "2017-05-30T18:53:27ZNOTCORRECT"}}}},
		{Key: "transactions.marks",
			Valid: []interface{}{obj{}, obj{tests.Str1024: obj{tests.Str1024: 21.0, "end": -45}}},
			Invalid: []tests.Invalid{
				{Msg: `marks/type`, Values: val{"marks"}},
				{Msg: `marks/patternproperties`, Values: val{
					obj{"timing": obj{"start": "start"}},
					obj{"timing": obj{"start": obj{}}},
					obj{"timing": obj{"m*e": -45}},
					obj{"timing": obj{"m\"": -45}},
					obj{"timing": obj{"m.": -45}}}},
				{Msg: `marks/additionalproperties`, Values: val{
					obj{"tim*ing": obj{"start": -45}},
					obj{"tim\"ing": obj{"start": -45}},
					obj{"tim.ing": obj{"start": -45}}}}}},
		{Key: "transactions.context.custom",
			Valid: val{obj{"whatever": obj{"comes": obj{"end": -45}}},
				obj{"whatever": 123}},
			Invalid: []tests.Invalid{
				{Msg: `context/properties/custom/additionalproperties`, Values: val{obj{"what.ever": 123}, obj{"what*ever": 123}, obj{"what\"ever": 123}}},
				{Msg: `context/properties/custom/type`, Values: val{"context"}}}},
		{Key: "transactions.context.request.body",
			Valid:   []interface{}{obj{}, tests.Str1025},
			Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/body/type`, Values: val{102}}}},
		{Key: "transactions.context.request.env",
			Valid:   []interface{}{obj{}},
			Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/env/type`, Values: val{102, "a"}}}},
		{Key: "transactions.context.request.cookies",
			Valid:   []interface{}{obj{}},
			Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/cookies/type`, Values: val{123, ""}}}},
		{Key: "transactions.context.tags",
			Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
			Invalid: []tests.Invalid{
				{Msg: `tags/type`, Values: val{"tags"}},
				{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
				{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}, obj{"invali.d": "hello"}}}}},
		{Key: "transactions.context.user.id",
			Valid: val{123, tests.Str1024Special},
			Invalid: []tests.Invalid{
				{Msg: `context/properties/user/properties/id/type`, Values: val{obj{}}},
				{Msg: `context/properties/user/properties/id/maxlength`, Values: val{tests.Str1025}}}},
	}...)
}
