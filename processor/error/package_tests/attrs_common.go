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
	"github.com/elastic/apm-server/model/error/generated/schema"
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/tests"
)

type obj = map[string]interface{}
type val = []interface{}

func procSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc:            &tests.V1TestProcessor{Processor: perr.Processor},
		FullPayloadPath: "../testdata/error/payload.json",
		TemplatePaths: []string{"../../../model/error/_meta/fields.yml",
			"../../../_meta/fields.common.yml"},
		Schema: schema.PayloadSchema,
	}
}

func payloadAttrsNotInFields(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		tests.Group("error.exception.attributes"),

		"error.exception.stacktrace",
		"error.log.stacktrace",
	))
}

func fieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"listening", "view errors", "error id icon",
		"context.user.user-agent", "context.user.ip", "context.system.ip",
		tests.Group("timestamp"),
	))
}

func payloadAttrsNotInJsonSchema(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"errors.log.stacktrace.vars.key",
		"errors.exception.stacktrace.vars.key",
		"errors.exception.attributes.foo",
		"errors.context.request.headers.some-other-header",
		"errors.context.request.headers.array",
		tests.Group("errors.context.custom"),
		tests.Group("errors.context.request.env"),
		tests.Group("errors.context.request.cookies"),
		tests.Group("errors.context.tags"),
	))
}

func requiredKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"errors",
		"errors.log",
		"errors.exception",
		"errors.log.message",
		"errors.exception.message",
		"errors.exception.type",
		"errors.exception.stacktrace.filename",
		"errors.exception.stacktrace.lineno",
		"errors.log.stacktrace.filename",
		"errors.log.stacktrace.lineno",
		"errors.context.request.method",
		"errors.context.request.url",
	))
}

func condRequiredKeys(c map[string]tests.Condition) map[string]tests.Condition {
	base := map[string]tests.Condition{
		"errors.exception":         tests.Condition{Absence: []string{"errors.log"}},
		"errors.exception.message": tests.Condition{Absence: []string{"errors.exception.type"}},
		"errors.exception.type":    tests.Condition{Absence: []string{"errors.exception.message"}},
		"errors.log":               tests.Condition{Absence: []string{"errors.exception"}},
	}
	for k, v := range c {
		base[k] = v
	}
	return base
}

func keywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"processor.event", "processor.name", "listening", "error.grouping_key",
		"error.id", "transaction.id", "context.tags", "parent.id", "trace.id",
		"view errors", "error id icon"))
}

func templateToSchemaMapping(mapping map[string]string) map[string]string {
	return map[string]string{
		"context.system.":  "system.",
		"context.process.": "process.",
		"context.service.": "service.",
		"context.request.": "errors.context.request.",
		"context.user.":    "errors.context.user.",
		"span.":            "errors.spans.",
		"error.":           "errors.",
		"trace.id":         "errors.trace.id",
	}
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
		{Key: "errors.id", Valid: val{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
			Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7", "0123456789abcdef"}}}},
		{Key: "errors.transaction.id",
			Valid: val{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
			Invalid: []tests.Invalid{{Msg: `transaction/properties/id/pattern`, Values: val{"123",
				"z5925e55-b43f-4340-a8e0-df1906ecbf7a", "z5925e55-b43f-4340-a8e0-df1906ecbf7ia"}}}},
		{Key: "service.name", Valid: val{tests.Str1024},
			Invalid: []tests.Invalid{{Msg: `service/properties/name`, Values: val{tests.Str1024Special, tests.Str1025}}}},
		{Key: "errors",
			Invalid: []tests.Invalid{{Msg: `errors/type`, Values: val{false}}, {Msg: `errors/minitems`, Values: val{[]interface{}{}}}}},
		{Key: "errors.exception.code", Valid: val{"success", ""},
			Invalid: []tests.Invalid{{Msg: `exception/properties/code/type`, Values: val{false}}}},
		{Key: "errors.exception.attributes", Valid: val{map[string]interface{}{}},
			Invalid: []tests.Invalid{{Msg: `exception/properties/attributes/type`, Values: val{123}}}},
		{Key: "errors.timestamp",
			Valid: val{"2017-05-30T18:53:42.281Z"},
			Invalid: []tests.Invalid{
				{Msg: `timestamp/format`, Values: val{"2017-05-30T18:53Z", "2017-05-30T18:53:27.Z", "2017-05-30T18:53:27a123Z"}},
				{Msg: `timestamp/pattern`, Values: val{"2017-05-30T18:53:27.000+00:20", "2017-05-30T18:53:27ZNOTCORRECT"}}}},
		{Key: "errors.log.stacktrace.post_context",
			Valid: val{[]interface{}{}, []interface{}{"context"}},
			Invalid: []tests.Invalid{
				{Msg: `log/properties/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
				{Msg: `log/properties/stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
		{Key: "errors.log.stacktrace.pre_context",
			Valid: val{[]interface{}{}, []interface{}{"context"}},
			Invalid: []tests.Invalid{
				{Msg: `log/properties/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
				{Msg: `log/properties/stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
		{Key: "errors.exception.stacktrace.post_context",
			Valid: val{[]interface{}{}, []interface{}{"context"}},
			Invalid: []tests.Invalid{
				{Msg: `exception/properties/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
				{Msg: `exception/properties/stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
		{Key: "errors.exception.stacktrace.pre_context",
			Valid: val{[]interface{}{}, []interface{}{"context"}},
			Invalid: []tests.Invalid{
				{Msg: `exception/properties/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
				{Msg: `exception/properties/stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
		{Key: "errors.context.custom",
			Valid: val{obj{"whatever": obj{"comes": obj{"end": -45}}}, obj{"whatever": 123}},
			Invalid: []tests.Invalid{
				{Msg: `context/properties/custom/additionalproperties`, Values: val{
					obj{"what.ever": 123}, obj{"what*ever": 123}, obj{"what\"ever": 123}}},
				{Msg: `context/properties/custom/type`, Values: val{"context"}}}},
		{Key: "errors.context.request.body", Valid: val{tests.Str1025, obj{}},
			Invalid: []tests.Invalid{{Msg: `/context/properties/request/properties/body/type`, Values: val{102}}}},
		{Key: "errors.context.request.env", Valid: val{obj{}},
			Invalid: []tests.Invalid{{Msg: `/context/properties/request/properties/env/type`, Values: val{102, "a"}}}},
		{Key: "errors.context.request.cookies", Valid: val{obj{}},
			Invalid: []tests.Invalid{{Msg: `/context/properties/request/properties/cookies/type`, Values: val{102, "a"}}}},
		{Key: "errors.context.tags",
			Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
			Invalid: []tests.Invalid{
				{Msg: `context/properties/tags/type`, Values: val{"tags"}},
				{Msg: `context/properties/tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
				{Msg: `context/properties/tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}, obj{"invali.d": "hello"}}}}},
		{Key: "errors.context.user.id", Valid: val{123, tests.Str1024Special},
			Invalid: []tests.Invalid{
				{Msg: `context/properties/user/properties/id/type`, Values: val{obj{}}},
				{Msg: `context/properties/user/properties/id/maxlength`, Values: val{tests.Str1025}}}},
	}...)
}
