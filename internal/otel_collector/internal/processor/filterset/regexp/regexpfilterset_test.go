// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package regexp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	validRegexpFilters = []string{
		"prefix/.*",
		"prefix_.*",
		".*/suffix",
		".*_suffix",
		".*/contains/.*",
		".*_contains_.*",
		"full/name/match",
		"full_name_match",
	}
)

func TestNewRegexpFilterSet(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		success bool
	}{
		{
			name:    "validFilters",
			filters: validRegexpFilters,
			success: true,
		}, {
			name: "invalidFilter",
			filters: []string{
				"exact_string_match",
				"(a|b))", // invalid regex
			},
			success: false,
		}, {
			name:    "emptyFilter",
			filters: []string{},
			success: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fs, err := NewFilterSet(test.filters, nil)
			assert.Equal(t, test.success, fs != nil)
			assert.Equal(t, test.success, err == nil)

			if err == nil {
				// sanity call
				fs.Matches("test")
			}
		})
	}
}

func TestRegexpMatches(t *testing.T) {
	fs, err := NewFilterSet(validRegexpFilters, &Config{})
	assert.NotNil(t, fs)
	assert.NoError(t, err)
	assert.False(t, fs.cacheEnabled)

	matches := []string{
		"full/name/match",
		"extra/full/name/match/extra",
		"full_name_match",
		"prefix/test/match",
		"prefix_test_match",
		"extra/prefix/test/match",
		"test/match/suffix",
		"test/match/suffixextra",
		"test_match_suffix",
		"test/contains/match",
		"test_contains_match",
	}

	for _, m := range matches {
		t.Run(m, func(t *testing.T) {
			assert.True(t, fs.Matches(m))
		})
	}

	mismatches := []string{
		"not_exact_string_match",
		"random",
		"c",
	}

	for _, m := range mismatches {
		t.Run(m, func(t *testing.T) {
			assert.False(t, fs.Matches(m))
		})
	}
}

func TestRegexpDeDup(t *testing.T) {
	dupRegexpFilters := []string{
		"prefix/.*",
		"prefix/.*",
	}
	fs, err := NewFilterSet(dupRegexpFilters, &Config{})
	assert.NotNil(t, fs)
	assert.NoError(t, err)
	assert.False(t, fs.cacheEnabled)
	assert.EqualValues(t, 1, len(fs.regexes))
}

func TestRegexpMatchesCaches(t *testing.T) {
	// 0 means unlimited cache
	fs, err := NewFilterSet(validRegexpFilters, &Config{
		CacheEnabled:       true,
		CacheMaxNumEntries: 0,
	})
	assert.NotNil(t, fs)
	assert.NoError(t, err)
	assert.True(t, fs.cacheEnabled)

	matches := []string{
		"full/name/match",
		"extra/full/name/match/extra",
		"full_name_match",
		"prefix/test/match",
		"prefix_test_match",
		"extra/prefix/test/match",
		"test/match/suffix",
		"test/match/suffixextra",
		"test_match_suffix",
		"test/contains/match",
		"test_contains_match",
	}

	for _, m := range matches {
		t.Run(m, func(t *testing.T) {
			assert.True(t, fs.Matches(m))

			matched, ok := fs.cache.Get(m)
			assert.True(t, matched.(bool) && ok)
		})
	}

	mismatches := []string{
		"not_exact_string_match",
		"random",
		"c",
	}

	for _, m := range mismatches {
		t.Run(m, func(t *testing.T) {
			assert.False(t, fs.Matches(m))

			matched, ok := fs.cache.Get(m)
			assert.True(t, !matched.(bool) && ok)
		})
	}
}

func TestWithCacheSize(t *testing.T) {
	size := 3
	fs, err := NewFilterSet(validRegexpFilters, &Config{
		CacheEnabled:       true,
		CacheMaxNumEntries: size,
	})
	assert.NotNil(t, fs)
	assert.NoError(t, err)

	matches := []string{
		"prefix/test/match",
		"prefix_test_match",
		"test/match/suffix",
	}

	// fill cache
	for _, m := range matches {
		fs.Matches(m)
		_, ok := fs.cache.Get(m)
		assert.True(t, ok)
	}

	// refresh oldest entry
	fs.Matches(matches[0])

	// cause LRU cache eviction
	newest := "new"
	fs.Matches(newest)

	_, evictedOk := fs.cache.Get(matches[1])
	assert.False(t, evictedOk)

	_, newOk := fs.cache.Get(newest)
	assert.True(t, newOk)
}
