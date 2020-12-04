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
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/sourcemap/test"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestMapNilConsumer(t *testing.T) {
	// no sourcemapConsumer
	_, _, _, _, _, _, _, ok := Map(nil, 0, 0)
	assert.False(t, ok)
}

func TestMapNoMatch(t *testing.T) {
	m, err := sourcemap.Parse("", []byte(test.ValidSourcemap))
	require.NoError(t, err)

	// nothing found for lineno and colno
	file, fc, line, col, ctxLine, _, _, ok := Map(m, 0, 0)
	require.False(t, ok)
	assert.Zero(t, file)
	assert.Zero(t, fc)
	assert.Zero(t, line)
	assert.Zero(t, col)
	assert.Zero(t, ctxLine)
}

func TestMapMatch(t *testing.T) {
	validSourcemap := test.ValidSourcemap

	// Re-encode the sourcemap, adding carriage returns to the
	// line endings in the source content.
	decoded := make(map[string]interface{})
	require.NoError(t, json.Unmarshal([]byte(validSourcemap), &decoded))
	sourceContent := decoded["sourcesContent"].([]interface{})
	for i := range sourceContent {
		sourceContentFile := sourceContent[i].(string)
		sourceContentFile = strings.Replace(sourceContentFile, "\n", "\r\n", -1)
		sourceContent[i] = sourceContentFile
	}
	crlfSourcemap, err := json.Marshal(decoded)
	require.NoError(t, err)

	// mapping found in minified sourcemap
	test := func(t *testing.T, source []byte) {
		m, err := sourcemap.Parse("", source)
		require.NoError(t, err)
		file, fc, line, col, ctxLine, preCtx, postCtx, ok := Map(m, 1, 7)
		require.True(t, ok)
		assert.Equal(t, "webpack:///bundle.js", file)
		assert.Equal(t, "", fc)
		assert.Equal(t, 1, line)
		assert.Equal(t, 9, col)
		assert.Equal(t, "/******/ (function(modules) { // webpackBootstrap", ctxLine)
		assert.Empty(t, preCtx)
		assert.NotZero(t, postCtx)
	}
	t.Run("unix_endings", func(t *testing.T) { test(t, []byte(validSourcemap)) })
	t.Run("windows_endings", func(t *testing.T) { test(t, crlfSourcemap) })
}
