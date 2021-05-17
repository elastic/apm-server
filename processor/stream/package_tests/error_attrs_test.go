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
		"error.grouping_name", // added by ingest node
		tests.Group("event"),
		tests.Group("observer"),
		tests.Group("user"),
		tests.Group("client"),
		tests.Group("source"),
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

func TestErrorPayloadAttrsMatchFields(t *testing.T) {
	errorProcSetup().PayloadAttrsMatchFields(t,
		errorPayloadAttrsNotInFields(),
		errorFieldsNotInPayloadAttrs())
}
