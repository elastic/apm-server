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

func transactionProcSetup() *tests.ProcessorSetup {
	return &tests.ProcessorSetup{
		Proc: &intakeTestProcessor{
			Processor: *stream.BackendProcessor(&config.Config{MaxEventSize: lrSize}),
		},
		FullPayloadPath: "../testdata/intake-v2/transactions.ndjson",
		SchemaPath:      "../../../docs/spec/v2/transaction.json",
		TemplatePaths: []string{
			"../../../model/transaction/_meta/fields.yml",
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
		"host.name",
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
		"transaction.experience.longtask.count",
		"transaction.experience.longtask.sum",
		"transaction.experience.longtask.max",
	)
}

func transactionKeywordExceptionKeys() *tests.Set {
	return tests.NewSet(
		"data_stream.type", "data_stream.dataset", "data_stream.namespace",
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
