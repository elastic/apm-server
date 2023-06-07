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
	"errors"
	"strings"

	"github.com/go-sourcemap/sourcemap"
)

const sourcemapContentSnippetSize = 5

// Map sourcemapping for given line and column and return values after sourcemapping
func Map(mapper *sourcemap.Consumer, lineno, colno uint32) (
	file string, function string, l uint32, c uint32,
	contextLine string, preContext []string, postContext []string, err error) {

	if mapper == nil {
		return
	}
	var ok bool
	var line int
	var col int
	file, function, line, col, ok = mapper.Source(int(lineno), int(colno))
	if !ok {
		err = errors.New("failed to retrieve original source")
		return
	}
	l = uint32(line)
	c = uint32(col)
	scanner := bufio.NewScanner(strings.NewReader(mapper.SourceContent(file)))

	var currentLine int
	for scanner.Scan() {
		currentLine++
		if currentLine == line {
			contextLine = scanner.Text()
		} else if abs(line-currentLine) <= sourcemapContentSnippetSize {
			if currentLine < line {
				preContext = append(preContext, scanner.Text())
			} else {
				postContext = append(postContext, scanner.Text())
			}
		} else if currentLine > line {
			// More than sourcemapContentSnippetSize lines past, we're done.
			break
		}
	}
	err = scanner.Err()
	return
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
