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
		URL        string
		CleanedURL string
	}{
		{URL: "!@#$%ˆ&*()", CleanedURL: "!@#$%ˆ&*()"}, //leads to parse error
		{URL: "", CleanedURL: "."},
		{URL: "a/c", CleanedURL: "a/c"},
		{URL: "a//c", CleanedURL: "a/c"},
		{URL: "a/c/.", CleanedURL: "a/c"},
		{URL: "a/c/b/..", CleanedURL: "a/c"},
		{URL: "/../a/c", CleanedURL: "/a/c"},
		{URL: "/../a/b/../././/c", CleanedURL: "/a/c"},
	}
	for idx, test := range testData {
		CleanedURL := CleanUrlPath(test.URL)
		assert.Equal(t, test.CleanedURL, CleanedURL, fmt.Sprintf("(%v): Expected %s, got %s", idx, test.CleanedURL, CleanedURL))
	}
}
