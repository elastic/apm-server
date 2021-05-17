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

func spanProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &intakeTestProcessor{
			Processor: *stream.BackendProcessor(&config.Config{MaxEventSize: lrSize}),
		},
		FullPayloadPath: "../testdata/intake-v2/spans.ndjson",
		SchemaPath:      "../../../docs/spec/v2/span.json",
		TemplatePaths: []string{
			"../../../model/span/_meta/fields.yml",
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

func transactionContext() *tests.Set {
	return tests.NewSet(
		tests.Group("context.user"),
		tests.Group("context.response"),
		tests.Group("context.request"),
	)
}

func spanKeywordExceptionKeys() *tests.Set {
	return tests.Union(tests.NewSet(
		"data_stream.type", "data_stream.dataset", "data_stream.namespace",
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
		tests.Group("client"),
		tests.Group("source"),
	),
		transactionContext(),
	)
}

func TestSpanPayloadMatchFields(t *testing.T) {
	spanProcSetup().PayloadAttrsMatchFields(t,
		spanPayloadAttrsNotInFields(),
		spanFieldsNotInPayloadAttrs())

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
