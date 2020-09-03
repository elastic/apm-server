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
	"github.com/elastic/apm-server/model/transaction/generated/schema"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func transactionProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &intakeTestProcessor{
			Processor: *stream.BackendProcessor(&config.Config{MaxEventSize: lrSize}),
		},
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
		tests.Group("context"),
		tests.Group("transaction.page"),
		tests.Group("http.request.cookies"),
		"transaction.message.body", "transaction.message.headers",
		"http.response.decoded_body_size", "http.response.encoded_body_size", "http.response.transfer_size",
	)
}

func transactionFieldsNotInPayloadAttrs() *tests.Set {
	return tests.NewSet(
		"context.http",
		"context.http.status_code",
		"host.ip",
		"transaction.duration.count",
		"transaction.marks.*.*",
		"source.ip",
		tests.Group("observer"),
		tests.Group("user"),
		tests.Group("client"),
		tests.Group("destination"),
		tests.Group("url"),
		tests.Group("http"),
		tests.Group("span"),
		tests.Group("transaction.self_time"),
		tests.Group("transaction.breakdown"),
		tests.Group("transaction.duration.sum"),
		"experimental",
	)
}

func transactionPayloadAttrsNotInJsonSchema() *tests.Set {
	return tests.NewSet(
		"transaction",
		tests.Group("transaction.context.request.env."),
		tests.Group("transaction.context.request.body"),
		tests.Group("transaction.context.request.cookies"),
		tests.Group("transaction.context.custom"),
		tests.Group("transaction.context.tags"),
		tests.Group("transaction.marks"),
		tests.Group("transaction.context.request.headers."),
		tests.Group("transaction.context.response.headers."),
		tests.Group("transaction.context.message.headers."),
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
		"processor.event", "processor.name",
		"transaction.marks",
		"context.tags",
		"event.outcome",
		tests.Group("observer"),
		tests.Group("url"),
		tests.Group("http"),
		tests.Group("destination"),

		// metadata fields
		tests.Group("agent"),
		tests.Group("container"),
		tests.Group("host"),
		tests.Group("kubernetes"),
		tests.Group("process"),
		tests.Group("service"),
		tests.Group("user"),
		tests.Group("span"),
		tests.Group("cloud"),
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
		tests.NewSet("transaction.context.user.email", "transaction.context.experimental", "transaction.sample_rate"))
}

func TestAttrsPresenceInTransaction(t *testing.T) {
	transactionProcSetup().AttrsPresence(t, transactionRequiredKeys(), nil)
}

func TestKeywordLimitationOnTransactionAttrs(t *testing.T) {
	transactionProcSetup().KeywordLimitation(
		t,
		transactionKeywordExceptionKeys(),
		[]tests.FieldTemplateMapping{
			{Template: "parent.id", Mapping: "parent_id"},
			{Template: "trace.id", Mapping: "trace_id"},
			{Template: "transaction.message.", Mapping: "context.message."},
			{Template: "transaction."},
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
				Valid: val{json.Number("1496170422281000")},
				Invalid: []tests.Invalid{
					{Msg: `timestamp/type`, Values: val{"1496170422281000"}}}},
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
			{Key: "transaction.context.request.headers", Valid: val{
				obj{"User-Agent": "go-1.1"},
				obj{"foo-bar": "a,b"},
				obj{"foo": []interface{}{"a", "b"}}},
				Invalid: []tests.Invalid{{Msg: `properties/headers`, Values: val{102, obj{"foo": obj{"bar": "a"}}}}}},
			{Key: "transaction.context.request.env",
				Valid:   []interface{}{obj{}},
				Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/env/type`, Values: val{102, "a"}}}},
			{Key: "transaction.context.request.cookies",
				Valid:   []interface{}{obj{}},
				Invalid: []tests.Invalid{{Msg: `context/properties/request/properties/cookies/type`, Values: val{123, ""}}}},
			{Key: "transaction.context.response.headers", Valid: val{
				obj{"User-Agent": "go-1.1"},
				obj{"foo-bar": "a,b"},
				obj{"foo": []interface{}{"a", "b"}}},
				Invalid: []tests.Invalid{{Msg: `properties/headers`, Values: val{102, obj{"foo": obj{"bar": "a"}}}}}},
			{Key: "transaction.context.tags",
				Valid: val{obj{tests.Str1024Special: tests.Str1024Special}, obj{tests.Str1024: 123.45}, obj{tests.Str1024: true}},
				Invalid: []tests.Invalid{
					{Msg: `tags/type`, Values: val{"tags"}},
					{Msg: `tags/patternproperties`, Values: val{obj{"invalid": tests.Str1025}, obj{tests.Str1024: obj{}}}},
					{Msg: `tags/additionalproperties`, Values: val{obj{"invali*d": "hello"}, obj{"invali\"d": "hello"}, obj{"invali.d": "hello"}}}}},
			{Key: "transaction.context.user.id",
				Valid: val{123, tests.Str1024Special},
				Invalid: []tests.Invalid{
					{Msg: `context/properties/user/properties/id/type`, Values: val{obj{}}},
					{Msg: `context/properties/user/properties/id/maxlength`, Values: val{tests.Str1025}}}},
		})
}
