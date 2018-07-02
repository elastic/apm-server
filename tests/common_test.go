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
	"testing"

	"github.com/stretchr/testify/assert"
)

const preIdx = 0
const postIdx = 1
const delimiterIdx = 2
const expectedIdx = 3

func TestStrConcat(t *testing.T) {
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
		newStr := StrConcat(dataRow[preIdx], dataRow[postIdx], dataRow[delimiterIdx])
		assert.Equal(t, dataRow[expectedIdx], newStr)
	}
}
