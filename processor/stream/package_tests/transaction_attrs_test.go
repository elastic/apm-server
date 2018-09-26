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
		SchemaPrefix:    "transaction",
		TemplatePaths: []string{
			"../../../model/transaction/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
	}
}

func transactionPayloadAttrsNotInFields() *tests.Set {
	return tests.NewSet(
		tests.Group("transaction.marks."),
		"transaction.span_count.started",
	)
}

func transactionFieldsNotInPayloadAttrs() *tests.Set {
	return tests.NewSet(
		"listening",
		"context.user.user-agent",
		"context.user.ip",
		"context.system.ip",
	)
}

func transactionPayloadAttrsNotInJsonSchema() *tests.Set {
	return tests.NewSet(
		"transaction",
		"transaction.context.request.headers.some-other-header",
		"transaction.context.request.headers.array",
		tests.Group("transaction.context.request.env."),
		tests.Group("transaction.context.request.body"),
		tests.Group("transaction.context.request.cookies"),
		tests.Group("transaction.context.custom"),
		tests.Group("transaction.context.tags"),
		tests.Group("transaction.marks"),
	)
}

func transactionRequiredKeys() *tests.Set {
	return tests.NewSet(
		"transaction",
		"transaction.span_count",
		"transaction.span_count.started",
		"transaction.trace_id",
		"transaction.id",
		"transaction.duration",
		"transaction.type",
		"transaction.context.request.method",
		"transaction.context.request.url",
	)
}

func transactionKeywordExceptionKeys() *tests.Set {
	return tests.NewSet(
		"processor.event", "processor.name", "listening",
		"transaction.marks",
		"context.tags",

		// metadata fields
		tests.Group("context.process"),
		tests.Group("context.service"),
		tests.Group("context.system"),
	)
}

func TestTransactionPayloadMatchFields(t *testing.T) {
	transactionProcSetup().PayloadAttrsMatchFields(t,
		transactionPayloadAttrsNotInFields(),
		transactionFieldsNotInPayloadAttrs())
}

func TestTransactionPayloadMatchJsonSchema(t *testing.T) {
	transactionProcSetup().PayloadAttrsMatchJsonSchema(t,
		transactionPayloadAttrsNotInJsonSchema(),
		nil)
}

func TestAttrsPresenceInTransaction(t *testing.T) {
	transactionProcSetup().AttrsPresence(t, transactionRequiredKeys(), nil)
}

func TestKeywordLimitationOnTransactionAttrs(t *testing.T) {
	transactionProcSetup().KeywordLimitation(
		t,
		transactionKeywordExceptionKeys(),
		map[string]string{
			"transaction.": "",
			"parent.id":    "parent_id",
			"trace.id":     "trace_id",
		},
	)
}

func TestPayloadDataForTransaction(t *testing.T) {
	// add test data for testing
	// * specific edge cases
	// * multiple allowed dataypes
	// * regex pattern, time formats
	// * length restrictions, other than keyword length restrictions

	transactionProcSetup().DataValidation(t,
		[]tests.SchemaTestData{
			{Key: "transaction.duration",
				Valid:   []interface{}{12.4},
				Invalid: []tests.Invalid{{Msg: `duration/type`, Values: val{"123"}}}},
			{Key: "transaction.timestamp",
				Valid: val{"2017-05-30T18:53:42.281Z"},
				Invalid: []tests.Invalid{
					{Msg: `timestamp/format`, Values: val{"2017-05-30T18:53Z", "2017-05-30T18:53:27.Z", "2017-05-30T18:53:27a123Z"}},
					{Msg: `timestamp/pattern`, Values: val{"2017-05-30T18:53:27.000+00:20", "2017-05-30T18:53:27ZNOTCORRECT"}}}},
			{Key: "transaction.marks",
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
			{Key: "transaction.context.custom",
				Valid: val{obj{"whatever": obj{"comes": obj{"end": -45}}},
					obj{"whatever": 123}},
				Invalid: []tests.Invalid{
					{Msg: `context/properties/custom/additionalproperties`, Values: val{obj{"what.ever": 123}, obj{"what*ever": 123}, obj{"what\"ever": 123}}},
					{Msg: `context/properties/custom/type`, Values: val{"context"}}}},
			{Key: "transaction.context.request.body",
				Valid:   []interface{}{obj{}, tests.Str1025},
				Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/body/type`, Values: val{102}}}},
			{Key: "transaction.context.request.env",
				Valid:   []interface{}{obj{}},
				Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/env/type`, Values: val{102, "a"}}}},
			{Key: "transaction.context.request.cookies",
				Valid:   []interface{}{obj{}},
				Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/cookies/type`, Values: val{123, ""}}}},
			{Key: "transaction.context.tags",
				Valid: val{obj{tests.Str1024Special: tests.Str1024Special}},
				Invalid: []tests.Invalid{
					{Msg: `tags/type`, Values: val{"tags"}},
					{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: 123}, obj{tests.Str1024: obj{}}}},
					{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}, obj{"invali.d": "hello"}}}}},
			{Key: "transaction.context.user.id",
				Valid: val{123, tests.Str1024Special},
				Invalid: []tests.Invalid{
					{Msg: `context/properties/user/properties/id/type`, Values: val{obj{}}},
					{Msg: `context/properties/user/properties/id/maxlength`, Values: val{tests.Str1025}}}},
		})
}
