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
		// "span.parent"
	))
}

func spanFieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"listening", "view spans", "context.user.user-agent",
		"context.user.ip", "context.system.ip",
		"span.parent", // from v1
		tests.Group("context.request"),
		tests.Group("context.service"),
		tests.Group("context.user"),
		tests.Group("context.process"),
		tests.Group("context.response"),
	))
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

func spanKeywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"processor.event", "processor.name", "listening",
		"transaction.marks",
		"context.tags",
		"hex_id",
		// tests.Group("context.user"),
		// tests.Group("context.request"),
		"context.user.email",
		"context.user.id",
		"context.user.username",
		"context.request.url.pathname",
		"context.request.url.protocol",
		"context.request.url.hash",
		"context.request.url.raw",
		"context.request.url.full",
		"context.request.url.hostname",
		"context.request.url.search",
		"context.request.url.port",
		"context.request.http_version",
		"context.request.method",
	))
}

func TestSpanPayloadMatchFields(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchFields(t,
		spanPayloadAttrsNotInFields(nil),
		spanFieldsNotInPayloadAttrs(metadataFields()))
}

func TestSpanPayloadMatchJsonSchema(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchJsonSchema(t,
		spanPayloadAttrsNotInJsonSchema(nil),
		spanJsonSchemaNotInPayloadAttrs(nil), "span")
}

func TestSpanAttrsPresenceInTransaction(t *testing.T) {
	spanProcSetup().AttrsPresence(t, spanRequiredKeys(nil), nil)
}

func TestSpanKeywordLimitationOnTransactionAttrs(t *testing.T) {
	spanProcSetup().KeywordLimitation(
		t,
		spanKeywordExceptionKeys(metadataFields()),
		map[string]string{
			"span.":          "",
			"transaction.id": "transaction_id",
			"span.hex_id":    "id",
		},
	)
}

// func TestPayloadDataForTransaction(t *testing.T) {
// 	// add test data for testing
// 	// * specific edge cases
// 	// * multiple allowed dataypes
// 	// * regex pattern, time formats
// 	// * length restrictions, other than keyword length restrictions

// 	spanProcSetup().DataValidation(t, schemaTestData(
// 		[]tests.SchemaTestData{
// 			{Key: "transactions.id",
// 				Valid:   []interface{}{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
// 				Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7"}}}},
// 			{Key: "transactions.spans", Valid: []interface{}{[]interface{}{}}},
// 			{Key: "transactions.spans.stacktrace.pre_context",
// 				Valid: val{[]interface{}{}, []interface{}{"context"}},
// 				Invalid: []tests.Invalid{
// 					{Msg: `/stacktrace/items/properties/pre_context/items/type`, Values: val{[]interface{}{123}}},
// 					{Msg: `stacktrace/items/properties/pre_context/type`, Values: val{"test"}}}},
// 			{Key: "transactions.spans.stacktrace.post_context",
// 				Valid: val{[]interface{}{}, []interface{}{"context"}},
// 				Invalid: []tests.Invalid{
// 					{Msg: `/stacktrace/items/properties/post_context/items/type`, Values: val{[]interface{}{123}}},
// 					{Msg: `stacktrace/items/properties/post_context/type`, Values: val{"test"}}}},
// 		}))
// }
