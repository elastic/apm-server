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

	"github.com/elastic/apm-server/model/transaction/generated/schema"

	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func transactionProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc:            &V2TestProcessor{StreamProcessor: stream.StreamProcessor{}},
		FullPayloadPath: "../testdata/intake-v2/transactions.ndjson",
		Schema:          schema.ModelSchema,
		TemplatePaths: []string{
			"../../../model/transaction/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
	}
}

func transactionPayloadAttrsNotInFields(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		tests.Group("transaction.marks."),
		tests.Group("context.db"),
		"span.stacktrace",
		"context.http",
		"context.http.url",
		"transaction.span_count.started",
	))
}

func transactionFieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"listening", "view spans", "context.user.user-agent",
		"context.user.ip", "context.system.ip"))
}

func transactionPayloadAttrsNotInJsonSchema(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"transaction",
		"transaction.context.request.headers.some-other-header",
		"transaction.context.request.headers.array",
		"transaction.spans.stacktrace.vars.key",
		tests.Group("transaction.context.request.env."),
		tests.Group("transaction.context.request.body"),
		tests.Group("transaction.context.request.cookies"),
		tests.Group("transaction.context.custom"),
		tests.Group("transaction.context.tags"),
		tests.Group("transaction.marks"),
	))
}

func transactionJsonSchemaNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"transactions.spans.transaction_id",
	))
}

func transactionRequiredKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"transaction",
		"transaction.span_count",
		"transaction.span_count.started",
		"transaction.trace_id",
		"transaction.id",
		"transaction.duration",
		"transaction.type",
		"transaction.context.request.method",
		"transaction.context.request.url",
	))
}

func transactionKeywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"processor.event", "processor.name", "listening",
		"transaction.marks",
		"context.tags",

		// metadata fields
		tests.Group("context.process"),
		tests.Group("context.service"),
		tests.Group("context.system"),
	))
}

func TestPayloadMatchFields(t *testing.T) {
	transactionProcSetup().PayloadAttrsMatchFields(t,
		transactionPayloadAttrsNotInFields(nil),
		transactionFieldsNotInPayloadAttrs(nil))
}

func TestPayloadMatchJsonSchema(t *testing.T) {
	transactionProcSetup().PayloadAttrsMatchJsonSchema(t,
		transactionPayloadAttrsNotInJsonSchema(nil),
		transactionJsonSchemaNotInPayloadAttrs(nil), "transaction")
}

func TestAttrsPresenceInTransaction(t *testing.T) {
	transactionProcSetup().AttrsPresence(t, transactionRequiredKeys(nil), nil)
}

func TestKeywordLimitationOnTransactionAttrs(t *testing.T) {
	transactionProcSetup().KeywordLimitation(
		t,
		transactionKeywordExceptionKeys(nil),
		map[string]string{
			"transaction.": "",
			"parent.id":    "parent_id",
			"trace.id":     "trace_id",
		},
	)
}

// func TestPayloadDataForTransaction(t *testing.T) {
// 	// add test data for testing
// 	// * specific edge cases
// 	// * multiple allowed dataypes
// 	// * regex pattern, time formats
// 	// * length restrictions, other than keyword length restrictions

// 	b.DataValidation(t, schemaTestData(
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
