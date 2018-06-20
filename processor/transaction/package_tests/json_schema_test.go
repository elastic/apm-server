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

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

//Check whether attributes are added to the example payload but not to the schema
func TestPayloadAttributesInSchema(t *testing.T) {

	//only add attributes that should not be documented by the schema
	undocumented := tests.NewSet(
		"transactions.spans.stacktrace.vars.key",
		"transactions.context.request.headers.some-other-header",
		"transactions.context.request.headers.array",
		"transactions.context.request.env.SERVER_SOFTWARE",
		"transactions.context.request.env.GATEWAY_INTERFACE",
		"transactions.context.request.body",
		"transactions.context.request.body.str",
		"transactions.context.request.body.additional",
		"transactions.context.request.body.additional.foo",
		"transactions.context.request.body.additional.bar",
		"transactions.context.request.body.additional.req",
		"transactions.context.request.cookies.c1",
		"transactions.context.request.cookies.c2",
		"transactions.context.custom",
		"transactions.context.custom.my_key",
		"transactions.context.custom.some_other_value",
		"transactions.context.custom.and_objects",
		"transactions.context.custom.and_objects.foo",
		"transactions.context.tags",
		"transactions.context.tags.organization_uuid",
		"transactions.marks.another_mark",
		"transactions.marks.another_mark.some_long",
		"transactions.marks.another_mark.some_float",
		"transactions.marks.navigationTiming",
		"transactions.marks.navigationTiming.appBeforeBootstrap",
		"transactions.marks.navigationTiming.navigationStart",
		"transactions.marks.performance",
	)
	tests.TestPayloadAttributesInSchema(t, "transaction", undocumented, transaction.Schema())
}

func TestJsonSchemaKeywordLimitation(t *testing.T) {
	fieldsPaths := []string{
		"./../../../_meta/fields.common.yml",
		"./../_meta/fields.yml",
	}
	exceptions := tests.NewSet("processor.event", "processor.name", "context.service.name", "transaction.id", "listening")
	tests.TestJsonSchemaKeywordLimitation(t, fieldsPaths, transaction.Schema(), exceptions)
}

func TestTransactionPayloadSchema(t *testing.T) {
	testData := []tests.SchemaTestData{
		{File: "data/invalid/transaction_payload/no_service.json", Error: "missing properties: \"service\""},
		{File: "data/invalid/transaction_payload/no_transactions.json", Error: "minimum 1 items allowed"},
	}
	tests.TestDataAgainstProcessor(t, transaction.NewProcessor(), testData)
}
