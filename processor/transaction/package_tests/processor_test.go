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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/tests/loader"
)

// ensure all valid documents pass through the whole validation and transformation process
func TestTransactionProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessTransactionFull", Path: "data/valid/transaction/payload.json"},
		{Name: "TestProcessTransactionNullValues", Path: "data/valid/transaction/null_values.json"},
		{Name: "TestProcessSystemNull", Path: "data/valid/transaction/system_null.json"},
		{Name: "TestProcessProcessNull", Path: "data/valid/transaction/process_null.json"},
		{Name: "TestProcessTransactionMinimalSpan", Path: "data/valid/transaction/minimal_span.json"},
		{Name: "TestProcessTransactionMinimalService", Path: "data/valid/transaction/minimal_service.json"},
		{Name: "TestProcessTransactionMinimalProcess", Path: "data/valid/transaction/minimal_process.json"},
		{Name: "TestProcessTransactionEmpty", Path: "data/valid/transaction/transaction_empty_values.json"},
		{Name: "TestProcessTransactionAugmentedIP", Path: "data/valid/transaction/augmented_payload_backend.json"},
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), config.Config{}, requestInfo, map[string]string{})
}

func TestMinimalTransactionProcessorOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessTransactionMinimalPayload", Path: "data/valid/transaction/minimal_payload.json"},
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), config.Config{}, requestInfo, map[string]string{"@timestamp": "-"})
}

func TestProcessorFrontendOK(t *testing.T) {
	requestInfo := []tests.RequestInfo{
		{Name: "TestProcessTransactionFrontend", Path: "data/valid/transaction/frontend.json"},
		{Name: "TestProcessTransactionAugmentedMerge", Path: "data/valid/transaction/augmented_payload_frontend.json"},
		{Name: "TestProcessTransactionAugmented", Path: "data/valid/transaction/augmented_payload_frontend_no_context.json"},
	}
	conf := config.Config{
		LibraryPattern:      regexp.MustCompile("/test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^~/test"),
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), conf, requestInfo, map[string]string{"@timestamp": "-"})
}

// ensure invalid documents fail the json schema validation already
func TestTransactionProcessorValidationFailed(t *testing.T) {
	data, err := loader.LoadInvalidData("transaction")
	assert.Nil(t, err)
	p := transaction.NewProcessor()
	err = p.Validate(data)
	assert.NotNil(t, err)
}
