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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/functionaltests/internal/esclient"
)

// CheckDocCountV7 checks if difference in index document count between
// current and previous state is equal to the expected, barring the skipped
// indices.
//
// NOTE: Ingestion should never remove documents. If the expectedDiff is
// negative, the index will be expected to appear, but the document count
// will not be asserted.
func CheckDocCountV7(
	t *testing.T,
	currDocCount, prevDocCount, expectedDiff esclient.IndicesDocCount,
) {
	t.Helper()

	if prevDocCount == nil {
		prevDocCount = currDocCount
	}

	// Check that all expected indices appear.
	for idx := range expectedDiff {
		if _, ok := currDocCount[idx]; !ok {
			t.Errorf("expected index %s not found", idx)
			continue
		}
	}

	// Check document counts for all indices.
	for idx, v := range currDocCount {
		e, ok := expectedDiff[idx]
		if !ok {
			t.Errorf("unexpected documents (%d) for index %s", v, idx)
		}

		if e < 0 {
			continue
		}

		assert.Equal(t, e, v-prevDocCount[idx],
			fmt.Sprintf("wrong document count difference for index %s", idx))
	}
}
