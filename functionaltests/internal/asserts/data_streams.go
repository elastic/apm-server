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

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type DataStreamExpectation struct {
	DSManagedBy      string
	PreferIlm        bool
	IndicesManagedBy []string
}

// DataStreamsMeetExpectation asserts that each data stream have expected values individually.
func DataStreamsMeetExpectation(t *testing.T, expected map[string]DataStreamExpectation, actual []types.DataStream) {
	t.Helper()

	assert.Len(t, actual, len(expected), "number of APM data streams differs from expectations")

	// Check that all expected data streams appear.
	mp := dataStreamsMap(actual)
	for ds := range expected {
		if _, ok := mp[ds]; !ok {
			t.Errorf("expected data stream %s not found", ds)
			continue
		}
	}

	// Check that data streams are in expected state.
	for _, v := range actual {
		e, ok := expected[v.Name]
		if !ok {
			t.Errorf("data stream %s not in expectation", v.Name)
			continue
		}

		checkSingleDataStream(t, e, v)
	}
}

func dataStreamsMap(dataStreams []types.DataStream) map[string]types.DataStream {
	result := make(map[string]types.DataStream)
	for _, dataStream := range dataStreams {
		result[dataStream.Name] = dataStream
	}
	return result
}

func checkSingleDataStream(t *testing.T, expected DataStreamExpectation, actual types.DataStream) {
	if expected.PreferIlm {
		assert.True(t, actual.PreferIlm, "data stream %s should prefer ILM", actual.Name)
	} else {
		assert.False(t, actual.PreferIlm, "data stream %s should not prefer ILM", actual.Name)
	}

	assert.Equal(t, expected.DSManagedBy, actual.NextGenerationManagedBy.Name,
		`data stream %s should be managed by "%s"`, actual.Name, expected.DSManagedBy,
	)

	assert.Len(t, actual.Indices, len(expected.IndicesManagedBy),
		"data stream %s should have %d indices", actual.Name, len(expected.IndicesManagedBy),
	)
	for i, index := range actual.Indices {
		assert.Equal(t, expected.IndicesManagedBy[i], index.ManagedBy.Name,
			`index %s should be managed by "%s"`, index.IndexName,
			expected.IndicesManagedBy[i],
		)
	}
}
