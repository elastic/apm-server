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

	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/transform"
)

var (
	backendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessTransactionFull", Path: "../testdata/transaction/payload.json"},
		{Name: "TestProcessTransactionNullValues", Path: "../testdata/transaction/null_values.json"},
		{Name: "TestProcessSystemNull", Path: "../testdata/transaction/system_null.json"},
		{Name: "TestProcessProcessNull", Path: "../testdata/transaction/process_null.json"},
		{Name: "TestProcessTransactionMinimalSpan", Path: "../testdata/transaction/minimal_span.json"},
		{Name: "TestProcessTransactionMinimalService", Path: "../testdata/transaction/minimal_service.json"},
		{Name: "TestProcessTransactionMinimalProcess", Path: "../testdata/transaction/minimal_process.json"},
		{Name: "TestProcessTransactionEmpty", Path: "../testdata/transaction/transaction_empty_values.json"},
		{Name: "TestProcessTransactionAugmentedIP", Path: "../testdata/transaction/augmented_payload_backend.json"},
	}

	backendRequestInfoIgnoreTimestamp = []tests.RequestInfo{
		{Name: "TestProcessTransactionMinimalPayload", Path: "../testdata/transaction/minimal_payload.json"},
	}

	rumRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessTransactionRum", Path: "../testdata/transaction/rum.json"},
		{Name: "TestProcessTransactionAugmentedMerge", Path: "../testdata/transaction/augmented_payload_rum.json"},
		{Name: "TestProcessTransactionAugmented", Path: "../testdata/transaction/augmented_payload_rum_no_context.json"},
	}
)

// ensure all valid documents pass through the whole validation and transformation process
func TestTransactionProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, transaction.Processor, transform.Context{}, backendRequestInfo, map[string]string{})
}

func TestMinimalTransactionProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, transaction.Processor, transform.Context{}, backendRequestInfoIgnoreTimestamp, map[string]string{"@timestamp": "-"})
}

func TestProcessorRumOK(t *testing.T) {
	tctx := transform.Context{
		Config: transform.Config{
			LibraryPattern:      regexp.MustCompile("/test/e2e|~"),
			ExcludeFromGrouping: regexp.MustCompile("^~/test"),
		},
	}
	tests.TestProcessRequests(t, transaction.Processor, tctx, rumRequestInfo, map[string]string{"@timestamp": "-"})
}

func BenchmarkBackendProcessor(b *testing.B) {
	tests.BenchmarkProcessRequests(b, transaction.Processor, transform.Context{}, backendRequestInfo)
	tests.BenchmarkProcessRequests(b, transaction.Processor, transform.Context{}, backendRequestInfoIgnoreTimestamp)
}

func BenchmarkRumProcessor(b *testing.B) {
	conf := transform.Config{
		LibraryPattern:      regexp.MustCompile("/test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^~/test"),
	}
	tctx := transform.Context{
		Config: conf,
	}
	tests.BenchmarkProcessRequests(b, transaction.Processor, tctx, rumRequestInfo)
}
