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

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model/span/generated/schema"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func spanProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &intakeTestProcessor{
			Processor: *stream.BackendProcessor(&config.Config{MaxEventSize: lrSize}),
		},
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
		tests.Group("span.stacktrace"),
		tests.Group("context"),
		tests.Group("span.db"),
		tests.Group("span.http"),
		"span.message.body", "span.message.headers",
	)
}

// fields in _meta/fields.common.yml that are shared between several data types, but not with spans
func spanFieldsNotInPayloadAttrs() *tests.Set {
	return tests.Union(
		tests.NewSet(
			"view spans",
			"transaction.sampled",
			"transaction.type",
			"transaction.name",
			tests.Group("container"),
			tests.Group("host"),
			tests.Group("kubernetes"),
			tests.Group("observer"),
			tests.Group("process"),
			tests.Group("service"),
			tests.Group("user"),
			tests.Group("client"),
			tests.Group("source"),
			tests.Group("http"),
			tests.Group("url"),
			tests.Group("span.self_time"),
			tests.Group("transaction.self_time"),
			tests.Group("transaction.breakdown"),
			tests.Group("transaction.duration"),
			"experimental",
		),
		// not valid for the span context
		transactionContext(),
	)

}

func spanPayloadAttrsNotInJsonSchema() *tests.Set {
	return tests.NewSet(
		"span",
		"span.stacktrace.vars.key",
		tests.Group("span.context.tags"),
		"span.context.http.response.headers.content-type",
		"span.context.service.environment", //used to check that only defined service fields are set on spans
	)
}

func spanJsonSchemaNotInPayloadAttrs() *tests.Set {
	return tests.NewSet(
		"span.transaction_id",
		"span.context.experimental",
		"span.context.message.body",
		"span.sample_rate",
		"span.context.message.headers",
	)
}

func spanRequiredKeys() *tests.Set {
	return tests.NewSet(
		"span",
		"span.name",
		"span.trace_id",
		"span.parent_id",
		"span.id",
		"span.duration",
		"span.type",
		"span.start",
		"span.timestamp",
		"span.stacktrace.filename",
	)
}

func spanCondRequiredKeys() map[string]tests.Condition {
	return map[string]tests.Condition{
		"span.start":     {Absence: []string{"span.timestamp"}},
		"span.timestamp": {Absence: []string{"span.start"}},

		"span.context.destination.service.type": {Existence: map[string]interface{}{
			"span.context.destination.service.name":     "postgresql",
			"span.context.destination.service.resource": "postgresql",
		}},
		"span.context.destination.service.name": {Existence: map[string]interface{}{
			"span.context.destination.service.type":     "db",
			"span.context.destination.service.resource": "postgresql",
		}},
		"span.context.destination.service.resource": {Existence: map[string]interface{}{
			"span.context.destination.service.type": "db",
			"span.context.destination.service.name": "postgresql",
		}},
	}
}

func transactionContext() *tests.Set {
	return tests.NewSet(
		tests.Group("context.user"),
		tests.Group("context.response"),
		tests.Group("context.request"),
	)
}

func spanKeywordExceptionKeys() *tests.Set {
	return tests.Union(tests.NewSet(
		"processor.event", "processor.name",
		"context.tags", "transaction.type", "transaction.name",
		"event.outcome",
		tests.Group("observer"),

		// metadata fields
		tests.Group("agent"),
		tests.Group("container"),
		tests.Group("host"),
		tests.Group("kubernetes"),
		tests.Group("process"),
		tests.Group("service"),
		tests.Group("user"),
		tests.Group("url"),
		tests.Group("http"),
		tests.Group("cloud"),
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
		[]tests.FieldTemplateMapping{
			{Template: "transaction.id", Mapping: "transaction_id"},
			{Template: "child.id", Mapping: "child_ids"},
			{Template: "parent.id", Mapping: "parent_id"},
			{Template: "trace.id", Mapping: "trace_id"},
			{Template: "span.id", Mapping: "id"},
			{Template: "span.db.link", Mapping: "context.db.link"},
			{Template: "span.destination.service", Mapping: "context.destination.service"},
			{Template: "span.message.", Mapping: "context.message."},
			{Template: "span.", Mapping: ""},
			{Template: "destination.address", Mapping: "context.destination.address"},
			{Template: "destination.port", Mapping: "context.destination.port"},
			{Template: "span.message.queue.name", Mapping: "context.message.queue.name"},
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
				Valid: val{obj{tests.Str1024Special: tests.Str1024Special}, obj{tests.Str1024: 123.45}, obj{tests.Str1024: true}},
				Invalid: []tests.Invalid{
					{Msg: `tags/type`, Values: val{"tags"}},
					{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: obj{}}}},
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
