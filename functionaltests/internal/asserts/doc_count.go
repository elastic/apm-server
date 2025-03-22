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

package asserts

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

// CheckDocCount checks if difference in data stream document count between current
// and previous state is equal to the expected, barring the skipped data streams.
func CheckDocCount(
	t *testing.T,
	currDocCount, prevDocCount, expectedDiff esclient.DataStreamsDocCount,
	skippedDataStreams []string,
) {
	t.Helper()

	sliceToMap := func(s []string) map[string]bool {
		m := make(map[string]bool)
		for _, v := range s {
			m[v] = true
		}
		return m
	}

	skipped := sliceToMap(skippedDataStreams)
	actualDiff := esclient.DataStreamsDocCount{}
	for ds, v := range currDocCount {
		if skipped[ds] {
			continue
		}

		_, ok := expectedDiff[ds]
		if !ok {
			t.Errorf("unexpected documents (%d) for %s", v, ds)
			continue
		}

		actualDiff[ds] = v - prevDocCount[ds]
	}

	assert.Equal(t, expectedDiff, actualDiff, "document count differences expectation not met")
}
