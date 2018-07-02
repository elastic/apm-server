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

package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDifferenceWithRegex(t *testing.T) {
	for idx, d := range []struct {
		s1, s2, diff *Set
	}{
		{nil, nil, nil},
		{NewSet("a.b.c", 2, 3), nil, NewSet("a.b.c", 2, 3)},
		{nil, NewSet("a.b.c", 2, 3), nil},
		{NewSet("a.b.c", 2), NewSet(2), NewSet("a.b.c")},
		{NewSet("a.b.c", 2), NewSet(Group("b")), NewSet("a.b.c", 2)},
		{NewSet("a.b.c", "ab", "a.c", "a.b.", 3), NewSet(Group("a.b.")), NewSet("ab", "a.c", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet(Group("a.b")), NewSet("a", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet("a.b"), NewSet("a", "a.b.c", 3)},
		{NewSet("a.b.c", "a", "a.b", 3), NewSet("a.b*"), NewSet("a", "a.b", "a.b.c", 3)},
	} {
		out := differenceWithGroup(d.s1, d.s2)
		assert.ElementsMatch(t, d.diff.Array(), out.Array(),
			fmt.Sprintf("Idx <%v>: Expected %v, Actual %v", idx, d.diff.Array(), out.Array()))
	}

}

func TestStrConcat(t *testing.T) {
	preIdx, postIdx, delimiterIdx, expectedIdx := 0, 1, 2, 3
	testData := [][]string{
		{"", "", "", ""},
		{"pre", "", "", "pre"},
		{"pre", "post", "", "prepost"},
		{"foo", "bar", ".", "foo.bar"},
		{"foo", "bar", ":", "foo:bar"},
		{"", "post", "", "post"},
		{"", "post", ",", "post"},
	}
	for _, dataRow := range testData {
		newStr := strConcat(dataRow[preIdx], dataRow[postIdx], dataRow[delimiterIdx])
		assert.Equal(t, dataRow[expectedIdx], newStr)
	}
}
