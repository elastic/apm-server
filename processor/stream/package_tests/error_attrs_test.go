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
	"github.com/elastic/apm-server/model/error/generated/schema"
	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func errorProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &intakeTestProcessor{
			Processor: *stream.BackendProcessor(&config.Config{MaxEventSize: lrSize}),
		},
		FullPayloadPath: "../testdata/intake-v2/errors.ndjson",
		TemplatePaths: []string{
			"../../../model/error/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
		Schema:       schema.ModelSchema,
		SchemaPrefix: "error",
	}
}

func errorPayloadAttrsNotInFields() *tests.Set {
	return tests.NewSet(
		tests.Group("error.exception.attributes"),
		tests.Group("error.exception.stacktrace"),
		tests.Group("error.exception.cause"),
		tests.Group("error.exception.parent"),
		tests.Group("error.log.stacktrace"),
		tests.Group("context"),
		tests.Group("error.page"),
		tests.Group("http.request.cookies"),
	)
}

func errorFieldsNotInPayloadAttrs() *tests.Set {
	return tests.NewSet(
		"view errors", "error id icon",
		"host.ip", "transaction.name", "source.ip",
		tests.Group("event"),
		tests.Group("observer"),
		tests.Group("user"),
		tests.Group("client"),
		tests.Group("destination"),
		tests.Group("http"),
		tests.Group("url"),
		tests.Group("span"),
		tests.Group("transaction.self_time"),
		tests.Group("transaction.breakdown"),
		tests.Group("transaction.duration"),
		"experimental",
	)
}

func errorPayloadAttrsNotInJsonSchema() *tests.Set {
	return tests.NewSet(
		"error",
		"error.log.stacktrace.vars.key",
		"error.exception.stacktrace.vars.key",
		"error.exception.attributes.foo",
		tests.Group("error.exception.cause."),
		tests.Group("error.context.custom"),
		tests.Group("error.context.request.env"),
		tests.Group("error.context.request.cookies"),
		tests.Group("error.context.tags"),
		tests.Group("error.context.request.headers."),
		tests.Group("error.context.response.headers."),
	)
}

func errorRequiredKeys() *tests.Set {
	return tests.NewSet(
		"error",
		"error.id",
		"error.log",
		"error.exception",
		"error.exception.type",
		"error.exception.message",
		"error.log.message",
		"error.exception.stacktrace.filename",
		"error.log.stacktrace.filename",
		"error.context.request.method",
		"error.trace_id",
		"error.parent_id",
	)
}

type val = []interface{}
type obj = map[string]interface{}

func errorCondRequiredKeys() map[string]tests.Condition {
	return map[string]tests.Condition{
		"error.exception":                      {Absence: []string{"error.log"}},
		"error.exception.message":              {Absence: []string{"error.exception.type"}},
		"error.exception.type":                 {Absence: []string{"error.exception.message"}},
		"error.exception.stacktrace.filename":  {Absence: []string{"error.exception.stacktrace.classname"}},
		"error.exception.stacktrace.classname": {Absence: []string{"error.exception.stacktrace.filename"}},
		"error.log":                            {Absence: []string{"error.exception"}},
		"error.log.stacktrace.filename":        {Absence: []string{"error.log.stacktrace.classname"}},
		"error.log.stacktrace.classname":       {Absence: []string{"error.log.stacktrace.filename"}},

		"error.trace_id":  {Existence: obj{"error.parent_id": "abc123"}},
		"error.parent_id": {Existence: obj{"error.trace_id": "abc123"}},
	}
}

func errorKeywordExceptionKeys() *tests.Set {
	return tests.NewSet(
		"data_stream.type", "data_stream.dataset", "data_stream.namespace",
		"processor.event", "processor.name", "error.grouping_key",
		"context.tags", "transaction.name",
		"event.outcome", // not relevant
		"view errors", "error id icon",
		tests.Group("url"),
		tests.Group("http"),
		tests.Group("destination"),
		// metadata field
		tests.Group("agent"),
		tests.Group("container"),
		tests.Group("host"),
		tests.Group("kubernetes"),
		tests.Group("observer"),
		tests.Group("process"),
		tests.Group("service"),
		tests.Group("user"),
		tests.Group("span"),
		tests.Group("cloud"),
	)
}

func TestErrorPayloadAttrsMatchFields(t *testing.T) {
	errorProcSetup().PayloadAttrsMatchFields(t,
		errorPayloadAttrsNotInFields(),
		errorFieldsNotInPayloadAttrs())
}

func TestErrorPayloadAttrsMatchJsonSchema(t *testing.T) {
	errorProcSetup().PayloadAttrsMatchJsonSchema(t,
		errorPayloadAttrsNotInJsonSchema(),
		tests.NewSet(
			"error.context.user.email",
			"error.context.experimental",
			"error.exception.parent", // it will never be present in the top (first) exception
			tests.Group("error.context.message"),
			"error.context.response.decoded_body_size",
			"error.context.response.encoded_body_size",
			"error.context.response.transfer_size",
		))
}

func TestErrorAttrsPresenceInError(t *testing.T) {
	errorProcSetup().AttrsPresence(t, errorRequiredKeys(), errorCondRequiredKeys())
}

func TestErrorKeywordLimitationOnErrorAttributes(t *testing.T) {
	errorProcSetup().KeywordLimitation(
		t,
		errorKeywordExceptionKeys(),
		[]tests.FieldTemplateMapping{
			{Template: "error."},
			{Template: "transaction.id", Mapping: "transaction_id"},
			{Template: "parent.id", Mapping: "parent_id"},
			{Template: "trace.id", Mapping: "trace_id"},
		},
	)
}

func TestPayloadDataForError(t *testing.T) {
	//// add test data for testing
	//// * specific edge cases
	//// * multiple allowed data types
	//// * regex pattern, time formats
	//// * length restrictions, other than keyword length restrictions
	errorProcSetup().DataValidation(t,
		[]tests.SchemaTestData{
			{Key: "error",
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{false}}}},
			{Key: "error.exception.code", Valid: val{"success", ""},
				Invalid: []tests.Invalid{{Msg: `validation error`, Values: val{false}}}},
			{Key: "error.exception.attributes", Valid: val{map[string]interface{}{}},
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{123}}}},
			{Key: "error.timestamp",
				Valid: val{json.Number("1496170422281000")},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{"1496170422281000"}}}},
			{Key: "error.log.stacktrace.post_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{[]interface{}{123}}},
					{Msg: `decode error`, Values: val{"test"}}}},
			{Key: "error.log.stacktrace.pre_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{[]interface{}{123}}},
					{Msg: `decode error`, Values: val{"test"}}}},
			{Key: "error.exception.stacktrace.post_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{[]interface{}{123}}},
					{Msg: `decode error`, Values: val{"test"}}}},
			{Key: "error.exception.stacktrace.pre_context",
				Valid: val{[]interface{}{}, []interface{}{"context"}},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{[]interface{}{123}}},
					{Msg: `decode error`, Values: val{"test"}}}},
			{Key: "error.context.custom",
				Valid: val{obj{"whatever": obj{"comes": obj{"end": -45}}}, obj{"whatever": 123}},
				Invalid: []tests.Invalid{
					{Msg: `validation error`, Values: val{
						obj{"what.ever": 123}, obj{"what*ever": 123}, obj{"what\"ever": 123}}},
					{Msg: `decode error`, Values: val{"context"}}}},
			{Key: "error.context.request.body", Valid: val{tests.Str1025, obj{}},
				Invalid: []tests.Invalid{{Msg: `validation error`, Values: val{102}}}},
			{Key: "error.context.request.headers", Valid: val{
				obj{"User-Agent": "go-1.1"},
				obj{"foo-bar": "a,b"},
				obj{"foo": []interface{}{"a", "b"}}},
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{102, obj{"foo": obj{"bar": "a"}}}}}},
			{Key: "error.context.response.headers", Valid: val{
				obj{"User-Agent": "go-1.1"},
				obj{"foo-bar": "a,b"},
				obj{"foo": []interface{}{"a", "b"}}},
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{102, obj{"foo": obj{"bar": "a"}}}}}},
			{Key: "error.context.request.env", Valid: val{obj{}},
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{102, "a"}}}},
			{Key: "error.context.request.cookies", Valid: val{obj{}},
				Invalid: []tests.Invalid{{Msg: `decode error`, Values: val{102, "a"}}}},
			{Key: "error.context.tags",
				Valid: val{obj{tests.Str1024Special: tests.Str1024Special}, obj{tests.Str1024: 123.45}, obj{tests.Str1024: true}},
				Invalid: []tests.Invalid{
					{Msg: `decode error`, Values: val{"tags"}},
					{Msg: `validation error`, Values: val{
						obj{"invalid": tests.Str1025},
						obj{tests.Str1024: obj{}},
						obj{"invali*d": "hello"},
						obj{"invali\"d": "hello"},
						obj{"invali.d": "hello"}}}}},
			{Key: "error.context.user.id", Valid: val{123, tests.Str1024Special},
				Invalid: []tests.Invalid{
					{Msg: `validation error`, Values: val{obj{}, tests.Str1025}}}},
		})
}
