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
	"strings"

	"github.com/go-sourcemap/sourcemap"
)

const sourcemapContentSnippetSize = 5
const sourcemapLineLimit = 256

// Map sourcemapping for given line and column and return values after sourcemapping
func Map(mapper *sourcemap.Consumer, lineno, colno int) (
	file string, function string, line int, col int,
	contextLine string, preContext []string, postContext []string, ok bool) {

	if mapper == nil {
		return
	}
	file, function, line, col, ok = mapper.Source(lineno, colno)
	src := strings.Split(mapper.SourceContent(file), "\n")
	contextLine = strings.Join(subSlice(line-1, line, src), "")
	preContext = subSlice(line-1-sourcemapContentSnippetSize, line-1, src)
	postContext = subSlice(line, line+sourcemapContentSnippetSize, src)
	return
}

func subSlice(from, to int, content []string) []string {
	if len(content) == 0 {
		return content
	}
	if from < 0 {
		from = 0
	}
	if to < 0 {
		to = 0
	}
	if from > len(content) {
		from = len(content)
	}
	if to > len(content) {
		to = len(content)
	}
	var slice = make([]string, to - from)
	for i, line := range content[from:to] {
		if len(line) <= sourcemapLineLimit {
			slice[i] = line
		} else {
			slice[i] = line[:sourcemapLineLimit-3] + "..."
		}
	}
	return slice
}
