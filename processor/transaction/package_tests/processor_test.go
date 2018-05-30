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

	"github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/processor/transaction"
	"github.com/elastic/apm-server/tests"
)

var (
	backendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessTransactionFull", Path: "data/transaction/payload.json"},
		{Name: "TestProcessTransactionNullValues", Path: "data/transaction/null_values.json"},
		{Name: "TestProcessSystemNull", Path: "data/transaction/system_null.json"},
		{Name: "TestProcessProcessNull", Path: "data/transaction/process_null.json"},
		{Name: "TestProcessTransactionMinimalSpan", Path: "data/transaction/minimal_span.json"},
		{Name: "TestProcessTransactionMinimalService", Path: "data/transaction/minimal_service.json"},
		{Name: "TestProcessTransactionMinimalProcess", Path: "data/transaction/minimal_process.json"},
		{Name: "TestProcessTransactionEmpty", Path: "data/transaction/transaction_empty_values.json"},
		{Name: "TestProcessTransactionAugmentedIP", Path: "data/transaction/augmented_payload_backend.json"},
	}

	backendRequestInfoIgnoreTimestamp = []tests.RequestInfo{
		{Name: "TestProcessTransactionMinimalPayload", Path: "data/transaction/minimal_payload.json"},
	}

	frontendRequestInfo = []tests.RequestInfo{
		{Name: "TestProcessTransactionFrontend", Path: "data/transaction/frontend.json"},
		{Name: "TestProcessTransactionAugmentedMerge", Path: "data/transaction/augmented_payload_frontend.json"},
		{Name: "TestProcessTransactionAugmented", Path: "data/transaction/augmented_payload_frontend_no_context.json"},
	}
)

// ensure all valid documents pass through the whole validation and transformation process
func TestTransactionProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, transaction.NewProcessor(), config.Config{}, backendRequestInfo, map[string]string{})
}

func TestMinimalTransactionProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, transaction.NewProcessor(), config.Config{}, backendRequestInfoIgnoreTimestamp, map[string]string{"@timestamp": "-"})
}

func TestProcessorFrontendOK(t *testing.T) {
	conf := config.Config{
		LibraryPattern:      regexp.MustCompile("/test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^~/test"),
	}
	tests.TestProcessRequests(t, transaction.NewProcessor(), conf, frontendRequestInfo, map[string]string{"@timestamp": "-"})
}

func BenchmarkBackendProcessor(b *testing.B) {
	tests.BenchmarkProcessRequests(b, transaction.NewProcessor(), config.Config{}, backendRequestInfo)
	tests.BenchmarkProcessRequests(b, transaction.NewProcessor(), config.Config{}, backendRequestInfoIgnoreTimestamp)
}

func BenchmarkFrontendProcessor(b *testing.B) {
	conf := config.Config{
		LibraryPattern:      regexp.MustCompile("/test/e2e|~"),
		ExcludeFromGrouping: regexp.MustCompile("^~/test"),
	}
	tests.BenchmarkProcessRequests(b, transaction.NewProcessor(), conf, frontendRequestInfo)
}
