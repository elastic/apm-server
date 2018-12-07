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

package utility

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUrlPath(t *testing.T) {
	testData := []struct {
		Url        string
		CleanedUrl string
	}{
		{Url: "!@#$%ˆ&*()", CleanedUrl: "!@#$%ˆ&*()"}, //leads to parse error
		{Url: "", CleanedUrl: "."},
		{Url: "a/c", CleanedUrl: "a/c"},
		{Url: "a//c", CleanedUrl: "a/c"},
		{Url: "a/c/.", CleanedUrl: "a/c"},
		{Url: "a/c/b/..", CleanedUrl: "a/c"},
		{Url: "/../a/c", CleanedUrl: "/a/c"},
		{Url: "/../a/b/../././/c", CleanedUrl: "/a/c"},
	}
	for idx, test := range testData {
		cleanedUrl := CleanUrlPath(test.Url)
		assert.Equal(t, test.CleanedUrl, cleanedUrl, fmt.Sprintf("(%v): Expected %s, got %s", idx, test.CleanedUrl, cleanedUrl))
	}
}

func TestInsertInMap(t *testing.T) {
	type M = map[string]interface{}
	testData := []struct {
		data   M
		key    string
		values M
		result M
	}{
		{
			nil,
			"a",
			M{"a": 1},
			nil,
		},
		{
			M{"a": 1},
			"",
			nil,
			M{"a": 1},
		},
		{
			M{"a": 1},
			"",
			M{},
			M{"a": 1},
		},
		{
			M{},
			"",
			M{"a": 1},
			M{},
		},
		{
			M{"a": 1},
			"b",
			M{"c": 2},
			M{"a": 1, "b": M{"c": 2}},
		},
		{
			M{"a": 1},
			"a",
			M{"b": 2},
			M{"a": 1},
		},
		{
			M{"a": M{"b": 1}},
			"a",
			M{"c": 2},
			M{"a": M{"b": 1, "c": 2}},
		},
	}
	for idx, test := range testData {
		InsertInMap(test.data, test.key, test.values)
		assert.Equal(t, test.result, test.data,
			fmt.Sprintf("At (%v): Expected %s, got %s", idx, test.result, test.data))
	}
}

func TestMergeMaps(t *testing.T) {
	dest := map[string]interface{}{
		"a": "1",
		"b": map[string]interface{}{
			"c": map[string]interface{}{
				"d": 2,
			},
		},
		"e": map[string]interface{}{
			"f": map[string]interface{}{},
		},
	}

	source := map[string]interface{}{
		"a": "2", // dest should not be overwritten
		"b": map[string]interface{}{
			"c": map[string]interface{}{
				"new-val": 4, // should be merged in
			},
		},
		"e": map[string]interface{}{
			"f": "new-val-2", // different type
			"new-key": map[string]interface{}{ // should be merged in
				"nested-new-val": 3,
			},
		},
		"new-top-level-key": 5,
	}

	expected := map[string]interface{}{
		"a": "1",
		"b": map[string]interface{}{
			"c": map[string]interface{}{
				"d":       2,
				"new-val": 4, // merged in
			},
		},
		"e": map[string]interface{}{
			"f": map[string]interface{}{},
			"new-key": map[string]interface{}{ // should be merged in
				"nested-new-val": 3,
			},
		},
		"new-top-level-key": 5,
	}

	MergeMaps(dest, source)
	assert.Equal(t, expected, dest)
}
