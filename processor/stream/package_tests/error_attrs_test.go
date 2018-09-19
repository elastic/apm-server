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

	"github.com/elastic/apm-server/model/error/generated/schema"

	"github.com/elastic/apm-server/processor/stream"
	"github.com/elastic/apm-server/tests"
)

func errorProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc:            &V2TestProcessor{StreamProcessor: stream.StreamProcessor{}},
		FullPayloadPath: "../testdata/intake-v2/errors.ndjson",
		TemplatePaths: []string{
			"../../../model/error/_meta/fields.yml",
			"../../../_meta/fields.common.yml",
		},
		Schema: schema.ModelSchema,
	}
}

func errorPayloadAttrsNotInFields(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"error.exception.attributes",
		"error.exception.attributes.foo",
		"error.exception.stacktrace",
		"error.log.stacktrace",
	))
}

func errorFieldsNotInPayloadAttrs(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"listening", "view errors", "error id icon",
		"context.user.user-agent", "context.user.ip", "context.system.ip",
	))
}

func errorPayloadAttrsNotInJsonSchema(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"error",
		"error.log.stacktrace.vars.key",
		"error.exception.stacktrace.vars.key",
		"error.exception.attributes.foo",
		"error.context.request.headers.some-other-header",
		"error.context.request.headers.array",
		tests.Group("error.context.custom"),
		tests.Group("error.context.request.env"),
		tests.Group("error.context.request.cookies"),
		tests.Group("error.context.tags"),
	))
}

func errorRequiredKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"error",
		"error.exception.message",
		"error.exception",
		"error.log",
		"error.log.type",
		"error.log.message",
		"error.exception.stacktrace.filename",
		"error.exception.stacktrace.lineno",
		"error.log.stacktrace.filename",
		"error.log.stacktrace.lineno",
		"error.context.request.method",
		"error.context.request.url",
	))
}

// func errorJsonSchemaNotInPayloadAttrs(s *tests.Set) *tests.Set {
// 	return tests.Union(s, tests.NewSet(
// 		"transactions.spans.transaction_id",
// 	))
// }

func errorCondRequiredKeys(c map[string]tests.Condition) map[string]tests.Condition {
	base := map[string]tests.Condition{
		"error.exception": tests.Condition{Absence: []string{"error.log"}},
		"error.log":       tests.Condition{Absence: []string{"error.exception"}},
	}
	for k, v := range c {
		base[k] = v
	}
	return base
}

func errorKeywordExceptionKeys(s *tests.Set) *tests.Set {
	return tests.Union(s, tests.NewSet(
		"processor.event", "processor.name", "listening", "error.grouping_key",
		"context.tags",
		"view errors", "error id icon",
	))
}

func TestErrorPayloadAttrsMatchFields(t *testing.T) {
	errorProcSetup().PayloadAttrsMatchFields(t,
		errorPayloadAttrsNotInFields(nil),
		errorFieldsNotInPayloadAttrs(tests.NewSet("error.parent_id", "error.trace_id")))
}

func TestErrorPayloadAttrsMatchJsonSchema(t *testing.T) {
	errorProcSetup().PayloadAttrsMatchJsonSchema(t,
		errorPayloadAttrsNotInJsonSchema(nil), tests.NewSet(nil), "error")
}

func TestErrorAttrsPresenceInError(t *testing.T) {
	errorProcSetup().AttrsPresence(t, errorRequiredKeys(nil), errorCondRequiredKeys(nil))
}

func TestErrorKeywordLimitationOnErrorAttributes(t *testing.T) {
	errorProcSetup().KeywordLimitation(
		t,
		errorKeywordExceptionKeys(metadataFields()),
		map[string]string{
			"error.":         "",
			"transaction.id": "transaction_id",
			"parent.id":      "parent_id",
			"trace.id":       "trace_id",
		},
	)
}

// func TestPayloadDataForError(t *testing.T) {
// 	//// add test data for testing
// 	//// * specific edge cases
// 	//// * multiple allowed dataypes
// 	//// * regex pattern, time formats
// 	//// * length restrictions, other than keyword length restrictions
// 	errorProcSetup().DataValidation(t, schemaTestData(nil))
// }
