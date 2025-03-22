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
	"slices"
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
func CheckDataStreams(t *testing.T, want CheckDataStreamsWant, dss []types.DataStream) {
	t.Helper()

	require.Len(t, dss, want.Quantity, "number of data streams differs from expectations")
	if want.IndicesPerDS != len(want.IndicesManagedBy) {
		// Panic if not equal since it's due to developer mistake.
		panic("IndicesPerDS and length of IndicesManagedBy is not equal")
	}

	getIndicesManagement := func(name string, indices []types.DataStreamIndex, wants []string) map[string]string {
		if wants != nil {
			if !assert.Equal(t, len(indices), len(wants),
				"length of indices per data stream in %s mismatch", name) {
				if len(wants) < len(indices) {
					// Extend the slice so that the following will not panic.
					wants = append(wants, slices.Repeat([]string{""}, len(indices)-len(wants))...)
				}
			}
		}

		res := map[string]string{}
		for i, index := range indices {
			if wants != nil {
				res[index.IndexName] = wants[i]
			} else {
				res[index.IndexName] = index.ManagedBy.Name
			}
		}
		return res
	}

	type check struct {
		DSManagedBy      string
		PreferIlm        bool
		IndicesPerDS     int
		IndicesManagedBy map[string]string
	}

	expected := map[string]check{}
	actual := map[string]check{}
	for _, ds := range dss {
		expected[ds.Name] = check{
			DSManagedBy:      want.DSManagedBy,
			PreferIlm:        want.PreferIlm,
			IndicesPerDS:     want.IndicesPerDS,
			IndicesManagedBy: getIndicesManagement(ds.Name, ds.Indices, want.IndicesManagedBy),
		}
		actual[ds.Name] = check{
			DSManagedBy:      ds.NextGenerationManagedBy.Name,
			IndicesPerDS:     len(ds.Indices),
			PreferIlm:        ds.PreferIlm,
			IndicesManagedBy: getIndicesManagement(ds.Name, ds.Indices, nil),
		}
	}

	assert.Equal(t, expected, actual, "data streams expectation not met")
}
