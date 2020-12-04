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

	"github.com/elastic/apm-server/beater/config"
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
		},
		SchemaPath: "../../../docs/spec/v2/error.json",
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
