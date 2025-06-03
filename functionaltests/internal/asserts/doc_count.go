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

// DocExistFor checks if documents exist for the expected data stream / index.
func DocExistFor(t *testing.T, currDocCount map[string]int, names []string) {
	t.Helper()

	for _, name := range names {
		if _, ok := currDocCount[name]; !ok {
			t.Errorf("expected %s not found", name)
			continue
		}
	}
}

// DocCountIncreased checks if current document counts for all data streams / indices
// increased from the previous.
func DocCountIncreased(t *testing.T, currDocCount, prevDocCount map[string]int) {
	t.Helper()

	if currDocCount == nil {
		currDocCount = esclient.DataStreamsDocCount{}
	}
	if prevDocCount == nil {
		prevDocCount = esclient.DataStreamsDocCount{}
	}

	// Check that document counts have increased for all data streams.
	for ds, currCount := range currDocCount {
		prevCount := prevDocCount[ds]
		assert.Greaterf(t, currCount, prevCount,
			"document count did not increase for data stream %s", ds)
	}
}

// DocCountStayedTheSame checks if current document counts for all data streams / indices
// stayed the same from the previous.
func DocCountStayedTheSame(t *testing.T, currDocCount, prevDocCount map[string]int) {
	t.Helper()

	if currDocCount == nil {
		currDocCount = esclient.DataStreamsDocCount{}
	}
	if prevDocCount == nil {
		prevDocCount = esclient.DataStreamsDocCount{}
	}

	// Check that document counts stayed the same for all data streams.
	for ds, currCount := range currDocCount {
		prevCount := prevDocCount[ds]
		assert.Equalf(t, currCount, prevCount,
			"document count changed for data stream %s", ds)
	}
}
