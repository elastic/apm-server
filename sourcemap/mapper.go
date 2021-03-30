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
	"bufio"
	"strings"

	"github.com/go-sourcemap/sourcemap"
)

const sourcemapContentSnippetSize = 5

// Map sourcemapping for given line and column and return values after sourcemapping
func Map(mapper *sourcemap.Consumer, lineno, colno int, maxLineLength int) (
	file, function string,
	line, col int,
	contextLine string,
	preContext, postContext []string,
	ok bool,
) {
	if mapper == nil {
		return
	}

	file, function, line, col, ok = mapper.Source(lineno, colno)
	scanner := bufio.NewScanner(strings.NewReader(mapper.SourceContent(file)))

	var currentLine int
	for scanner.Scan() {
		currentLine++

		token := scanner.Text()
		if maxLineLength > 0 && len(token) > maxLineLength {
			token = truncate(token, maxLineLength)
		}

		if currentLine == line {
			contextLine = token
		} else if abs(line-currentLine) <= sourcemapContentSnippetSize {
			if currentLine < line {
				preContext = append(preContext, token)
			} else {
				postContext = append(postContext, token)
			}
		} else if currentLine > line {
			// More than sourcemapContentSnippetSize lines past, we're done.
			break
		}
	}
	if scanner.Err() != nil {
		ok = false
		return
	}
	return
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// truncate sourcemap lines by runes
func truncate(s string, length int) string {
	var j int
	for i := range s {
		if j == length {
			if length > 2 {
				return s[:i-2] + ".."
			}
			return s[:i]
		}
		j++
	}
	return s
}
