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

	"github.com/elastic/apm-server/model/span/generated/schema"

	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func spanProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc:            &V2TestProcessor{StreamProcessor: stream.StreamProcessor{}},
		FullPayloadPath: "../testdata/intake-v2/spans.ndjson",
		Schema:          schema.ModelSchema,
		TemplatePaths: []string{
			"../../../model/span/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
	}
}

func spanPayloadAttrsNotInFields(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"span.stacktrace",
		tests.Group("transaction.marks."),
		tests.Group("context.db"),
		"context.http",
		"context.http.url",
		"context.tags.tag1",
	))
}

func spanFieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.Union(tests.NewSet(
		"listening",
		"view spans",
		"span.parent", // from v1

		// not valid for the span context
	), transactionContext()))
}

func spanPayloadAttrsNotInJsonSchema(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"span",
		"span.context.request.headers.some-other-header",
		"span.context.request.headers.array",
		"span.stacktrace.vars.key",
		"span.context.tags.tag1",
		tests.Group("metadata"),
	))
}

func spanJsonSchemaNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"span.transaction_id",
	))
}

func spanRequiredKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"span",
		"span.name",
		"span.trace_id",
		"span.parent_id",
		"span.transaction_id",
		"span.id",
		"span.duration",
		"span.type",
		"span.start",
		"span.stacktrace.filename",
		"span.stacktrace.lineno",
		"span.context.request.method",
		"span.context.request.url",
	))
}

func transactionContext() *tests.Set {
	return tests.NewSet(
		tests.Group("context.request.url"),
		tests.Group("context.service"),
		tests.Group("context.user"),
		tests.Group("context.process"),
		tests.Group("context.response"),
		tests.Group("context.request"),
		tests.Group("context.system"),
	)
}

func spanKeywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.Union(tests.NewSet(
		"processor.event", "processor.name", "listening",
		"context.tags",
	), transactionContext()))
}

func TestSpanPayloadMatchFields(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchFields(t,
		spanPayloadAttrsNotInFields(nil),
		spanFieldsNotInPayloadAttrs(nil))

}

func TestSpanPayloadMatchJsonSchema(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchJsonSchema(t,
		spanPayloadAttrsNotInJsonSchema(nil),
		spanJsonSchemaNotInPayloadAttrs(nil), "span")
}

func TestAttrsPresenceInSpan(t *testing.T) {
	spanProcSetup().AttrsPresence(t, spanRequiredKeys(nil), nil)
}

func TestKeywordLimitationOnSpanAttrs(t *testing.T) {
	spanProcSetup().KeywordLimitation(
		t,
		spanKeywordExceptionKeys(nil),
		map[string]string{
			"transaction.id": "transaction_id",
			"parent.id":      "parent_id",
			"trace.id":       "trace_id",
			"span.hex_id":    "id",
			"span.name":      "name",
			"span.type":      "type",
		},
	)
}

func TestPayloadDataForSpans(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	spanProcSetup().DataValidation(t,
		[]tests.SchemaTestData{
			{Key: "span.stacktrace.pre_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
			{Key: "span.stacktrace.post_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
					{Msg: `stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
		})
}
