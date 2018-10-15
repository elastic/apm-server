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
	"encoding/json"
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
		SchemaPrefix:    "span",
		TemplatePaths: []string{
			"../../../model/span/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
	}
}

func spanPayloadAttrsNotInFields() *tests.Set {
	return tests.NewSet(
		"span.stacktrace",
		tests.Group("context.db"),
		"context.http",
		"context.http.url",
		"context.tags.tag1",
	)
}

func spanFieldsNotInPayloadAttrs() *tests.Set {
	return tests.Union(
		tests.NewSet(
			"listening",
			"view spans",
			"span.parent", // from v1
		),
		// not valid for the span context
		transactionContext(),
	)

}

func spanPayloadAttrsNotInJsonSchema() *tests.Set {
	return tests.NewSet(
		"span",
		"span.stacktrace.vars.key",
		"span.context.tags.tag1",
	)
}

func spanJsonSchemaNotInPayloadAttrs() *tests.Set {
	return tests.NewSet(
		"span.transaction_id",
	)
}

func spanRequiredKeys() *tests.Set {
	return tests.NewSet(
		"span",
		"span.name",
		"span.trace_id",
		"span.parent_id",
		"span.transaction_id",
		"span.id",
		"span.duration",
		"span.type",
		"span.start",
		"span.timestamp",
		"span.stacktrace.filename",
		"span.stacktrace.lineno",
	)
}

func spanCondRequiredKeys() map[string]tests.Condition {
	return map[string]tests.Condition{
		"span.start":     tests.Condition{Absence: []string{"span.timestamp"}},
		"span.timestamp": tests.Condition{Absence: []string{"span.start"}},
	}
}

func transactionContext() *tests.Set {
	return tests.NewSet(
		tests.Group("context.service"),
		tests.Group("context.user"),
		tests.Group("context.process"),
		tests.Group("context.response"),
		tests.Group("context.request"),
		tests.Group("context.system"),
	)
}

func spanKeywordExceptionKeys() *tests.Set {
	return tests.Union(tests.NewSet(
		"processor.event", "processor.name", "listening",
		"context.tags",
	),
		transactionContext(),
	)
}

func TestSpanPayloadMatchFields(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchFields(t,
		spanPayloadAttrsNotInFields(),
		spanFieldsNotInPayloadAttrs())

}

func TestSpanPayloadMatchJsonSchema(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchJsonSchema(t,
		spanPayloadAttrsNotInJsonSchema(),
		spanJsonSchemaNotInPayloadAttrs())
}

func TestAttrsPresenceInSpan(t *testing.T) {
	spanProcSetup().AttrsPresence(t, spanRequiredKeys(), spanCondRequiredKeys())
}

func TestKeywordLimitationOnSpanAttrs(t *testing.T) {
	spanProcSetup().KeywordLimitation(
		t,
		spanKeywordExceptionKeys(),
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
			{Key: "span.context.tags",
				Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
				Invalid: []tests.Invalid{
					{Msg: `tags/type`, Values: val{"tags"}},
					{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
					{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}, obj{"invali.d": "hello"}}}},
			},
			{Key: "span.timestamp",
				Valid: val{json.Number("1496170422281000")},
				Invalid: []tests.Invalid{
					{Msg: `timestamp/type`, Values: val{"1496170422281000"}}}},
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
