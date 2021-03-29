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
	"bytes"
	"strings"

	"github.com/go-sourcemap/sourcemap"
)

const (
	sourcemapContentSnippetSize = 5
	// TODO: What value do we want here?
	minMaxLineLength = 16
)

// Map sourcemapping for given line and column and return values after sourcemapping
func Map(mapper *sourcemap.Consumer, lineno, colno int, maxLineLength uint) (
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
	if maxLineLength > 0 {
		if maxLineLength < minMaxLineLength {
			maxLineLength = minMaxLineLength
		}
		scanner.Split(limitScanLines(int(maxLineLength)))
	}

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

// limitScanLines is a modified form of bufio.ScanLines. It truncates any lines
// longer than the supplied `max` value.
func limitScanLines(max int) func([]byte, bool) (int, []byte, error) {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			if i > max {
				for j := max; j > 0 && j >= max-2; j-- {
					data[j] = '.'
				}
				return i + 1, data[0:max], nil
			}
			// We have a full newline-terminated line that is smaller than `max`.
			return i + 1, dropCR(data[0:i]), nil
		}

		// If we're at EOF, we have a final, non-terminated line.
		if atEOF {
			return len(data), dropCR(data), nil
		}

		// Request more data.
		return 0, nil, nil
	}
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}

	return data
}
