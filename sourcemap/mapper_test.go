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

package sourcemap

import (
	"testing"
	"strings"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/sourcemap/test"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestApply(t *testing.T) {
	// no sourcemapConsumer
	_, _, _, _, _, _, _, ok := Map(nil, 0, 0)
	assert.False(t, ok)

	m, err := sourcemap.Parse("", []byte(test.ValidSourcemap))
	require.NoError(t, err)

	t.Run("notOK", func(t *testing.T) {
		// nothing found for lineno and colno
		file, fc, line, col, ctxLine, _, _, ok := Map(m, 0, 0)
		require.False(t, ok)
		assert.Zero(t, file)
		assert.Zero(t, fc)
		assert.Zero(t, line)
		assert.Zero(t, col)
		assert.Zero(t, ctxLine)
	})

	t.Run("OK", func(t *testing.T) {
		// mapping found in minified sourcemap
		file, fc, line, col, ctxLine, preCtx, postCtx, ok := Map(m, 1, 7)
		require.True(t, ok)
		assert.Equal(t, "webpack:///bundle.js", file)
		assert.Equal(t, "", fc)
		assert.Equal(t, 1, line)
		assert.Equal(t, 9, col)
		assert.Equal(t, "/******/ (function(modules) { // webpackBootstrap", ctxLine)
		assert.Equal(t, []string{}, preCtx)
		assert.NotZero(t, postCtx)
	})
}

func TestSubSlice(t *testing.T) {
	src := []string{"a", "b", "c", "d", "e", "f"}
	for _, test := range []struct {
		start, end int
		rs         []string
	}{
		{2, 4, []string{"c", "d"}},
		{-1, 1, []string{"a"}},
		{4, 10, []string{"e", "f"}},
		// relevant test cases because we don't control the input and can not panic
		{-5, -3, []string{}},
		{8, 10, []string{}},
	} {
		assert.Equal(t, test.rs, subSlice(test.start, test.end, src))
	}

	for _, test := range []struct {
		start, end int
	}{
		{0, 1},
		{0, 0},
		{-1, 0},
	} {
		assert.Equal(t, []string{}, subSlice(test.start, test.end, []string{}))
	}

	assert.Equal(t, []string{strings.Repeat("x", 253) + "..."}, subSlice(0, 1, []string{strings.Repeat("x", 257)}))
}
