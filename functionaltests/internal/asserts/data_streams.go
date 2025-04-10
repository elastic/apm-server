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
	"github.com/stretchr/testify/require"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

type CheckDataStreamsWant struct {
	Quantity         int
	DSManagedBy      string
	PreferIlm        bool
	IndicesPerDS     int
	IndicesManagedBy []string
}

// CheckDataStreams asserts expected values on specific data streams.
func CheckDataStreams(t *testing.T, expected CheckDataStreamsWant, actual []types.DataStream) {
	t.Helper()

	// Preliminarily check that these two are matching, to avoid panic later.
	require.Len(t, expected.IndicesManagedBy, expected.IndicesPerDS,
		"length of IndicesManagedBy should be equal to IndicesPerDS")
	assert.Len(t, actual, expected.Quantity, "number of APM data streams differs from expectations")

	for _, v := range actual {
		if expected.PreferIlm {
			assert.True(t, v.PreferIlm, "data stream %s should prefer ILM", v.Name)
		} else {
			assert.False(t, v.PreferIlm, "data stream %s should not prefer ILM", v.Name)
		}

		assert.Equal(t, expected.DSManagedBy, v.NextGenerationManagedBy.Name,
			`data stream %s should be managed by "%s"`, v.Name, expected.DSManagedBy,
		)
		assert.Len(t, v.Indices, expected.IndicesPerDS,
			"data stream %s should have %d indices", v.Name, expected.IndicesPerDS,
		)
		for i, index := range v.Indices {
			assert.Equal(t, expected.IndicesManagedBy[i], index.ManagedBy.Name,
				`index %s should be managed by "%s"`, index.IndexName,
				expected.IndicesManagedBy[i],
			)
		}
	}
}

type CheckDataStreamIndividualWant struct {
	DSManagedBy      string
	PreferIlm        bool
	IndicesManagedBy []string
}

func CheckDataStreamsIndividually(t *testing.T, expected map[string]CheckDataStreamIndividualWant, actual []types.DataStream) {
	t.Helper()

	assert.Len(t, actual, len(expected), "number of APM data streams differs from expectations")

	for _, v := range actual {
		e, ok := expected[v.Name]
		if !ok {
			t.Errorf("data stream %s not in expectation", v.Name)
			continue
		}

		if e.PreferIlm {
			assert.True(t, v.PreferIlm, "data stream %s should prefer ILM", v.Name)
		} else {
			assert.False(t, v.PreferIlm, "data stream %s should not prefer ILM", v.Name)
		}

		assert.Equal(t, e.DSManagedBy, v.NextGenerationManagedBy.Name,
			`data stream %s should be managed by "%s"`, v.Name, e.DSManagedBy,
		)
		assert.Len(t, v.Indices, len(e.IndicesManagedBy),
			"data stream %s should have %d indices", v.Name, len(e.IndicesManagedBy),
		)
		for i, index := range v.Indices {
			assert.Equal(t, e.IndicesManagedBy[i], index.ManagedBy.Name,
				`index %s should be managed by "%s"`, index.IndexName,
				e.IndicesManagedBy[i],
			)
		}
	}
}
