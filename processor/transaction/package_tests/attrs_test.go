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

	"github.com/elastic/apm-server/tests"
)

func TestPayloadMatchFields(t *testing.T) {
	procSetup().PayloadAttrsMatchFields(t,
		payloadAttrsNotInFields(nil),
		fieldsNotInPayloadAttrs(tests.NewSet("parent", "parent.id", "span.hex_id", "trace", "trace.id")))
}

func TestPayloadMatchJsonSchema(t *testing.T) {
	procSetup().PayloadAttrsMatchJsonSchema(t,
		payloadAttrsNotInJsonSchema(nil),
		jsonSchemaNotInPayloadAttrs(nil))
}

func TestAttrsPresenceInTransaction(t *testing.T) {
	procSetup().AttrsPresence(t, requiredKeys(nil), condRequiredKeys(nil))
}

func TestKeywordLimitationOnTransactionAttrs(t *testing.T) {
	procSetup().KeywordLimitation(t, keywordExceptionKeys(nil), templateToSchemaMapping(nil))
}

func TestPayloadDataForTransaction(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	procSetup().DataValidation(t, schemaTestData(
		[]tests.SchemaTestData{
			{Key: "transactions.id",
				Valid:   []interface{}{"85925e55-B43f-4340-a8e0-df1906ecbf7a"},
				Invalid: []tests.Invalid{{Msg: `id/pattern`, Values: val{"123", "z5925e55-b43f-4340-a8e0-df1906ecbf7a", "85925e55-b43f-4340-a8e0-df1906ecbf7"}}}},
			{Key: "transactions.spans", Valid: []interface{}{[]interface{}{}}},
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
