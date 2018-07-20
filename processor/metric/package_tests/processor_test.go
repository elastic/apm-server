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

	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/transform"
)

var (
	requestInfo = []tests.RequestInfo{
		{Name: "TestProcessMetric", Path: "../testdata/metric/payload.json"},
		{Name: "TestProcessMetricMinimal", Path: "../testdata/metric/minimal.json"},
		{Name: "TestProcessMetricMultipleSamples", Path: "../testdata/metric/multiple-samples.json"},
	}
)

func TestMetricProcessorOK(t *testing.T) {
	tests.TestProcessRequests(t, metric.Processor, transform.Context{}, requestInfo, map[string]string{})
}

func BenchmarkProcessor(b *testing.B) {
	tests.BenchmarkProcessRequests(b, metric.Processor, transform.Context{}, requestInfo)
}
