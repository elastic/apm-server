// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package package_tests

import (
	"testing"

	tr "github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

var (
	procSetup = tests.ProcessorSetup{
		Proc:            tr.NewProcessor(),
		FullPayloadPath: "data/transaction/payload.json",
		TemplatePaths: []string{"../_meta/fields.yml",
			"../../../_meta/fields.common.yml"},
	}
)

func TestAttributesPresenceInTransaction(t *testing.T) {
	requiredKeys := tests.NewSet(
		"transactions",
		"transactions.id",
		"transactions.duration",
		"transactions.type",
		"transactions.context.request.method",
		"transactions.context.request.url",
		"transactions.spans.duration",
		"transactions.spans.name",
		"transactions.spans.start",
		"transactions.spans.type",
		"transactions.spans.stacktrace.filename",
		"transactions.spans.stacktrace.lineno",
	)
	condRequiredKeys := map[string]tests.Condition{
		"transactions.spans.id": tests.Condition{Existence: map[string]interface{}{"transactions.spans.parent": float64(123)}},
	}
	procSetup.AttrsPresence(t, requiredKeys, condRequiredKeys)
}

func TestKeywordLimitationOnTransactionAttributes(t *testing.T) {
	keywordExceptionKeys := tests.NewSet(
		"processor.event", "processor.name", "listening",
		"transaction.id", "transaction.marks", "context.tags")

	mapping := map[string]string{
		"context.system.":  "system.",
		"context.process.": "process.",
		"context.service.": "service.",
		"context.request.": "transactions.context.request.",
		"context.user.":    "transactions.context.user.",
		"span.":            "transactions.spans.",
		"transaction.":     "transactions.",
	}
	procSetup.KeywordLimitation(t, keywordExceptionKeys, mapping)
}

func TestPayloadDataForTransaction(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	type obj = map[string]interface{}
	type val = []interface{}

	payloadData := []tests.SchemaTestData{
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
	}
	procSetup.DataValidation(t, payloadData)
}

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	//only add attributes that should not be documented by the schema
	undocumented := tests.NewSet(
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
		"transactions.marks.another_mark",
		"transactions.marks.another_mark.some_long",
		"transactions.marks.another_mark.some_float",
		"transactions.marks.navigationTiming",
		"transactions.marks.navigationTiming.appBeforeBootstrap",
		"transactions.marks.navigationTiming.navigationStart",
		"transactions.marks.performance",
	)
	tests.TestPayloadAttributesInSchema(t, "transaction", undocumented, tr.Schema())
}
