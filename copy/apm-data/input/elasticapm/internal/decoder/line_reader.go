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

package decoder

import (
	"bufio"
	"io"

	"github.com/pkg/errors"
)

var ErrLineTooLong = errors.New("Line exceeded permitted length")

// LineReader reads length-limited lines from streams using a limited amount of memory.
type LineReader struct {
	br            *bufio.Reader
	maxLineLength int
	skip          bool
}

func NewLineReader(reader *bufio.Reader, maxLineLength int) *LineReader {
	return &LineReader{
		br:            reader,
		maxLineLength: maxLineLength,
	}
}

// Reset sets lr's underlying *bufio.Reader to br, and clears any state.
func (lr *LineReader) Reset(br *bufio.Reader) {
	lr.br = br
	lr.skip = false
}

// ReadLine reads the next line from the given reader.
// If it encounters a line that is longer than `maxLineLength` it will
// return the first `maxLineLength` bytes with `ErrLineTooLong`. On the next
// call it will return the next line.
func (lr *LineReader) ReadLine() ([]byte, error) {
	for {
		prefix := false
		line, err := lr.br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			prefix = true
		}

		if !lr.skip {
			if prefix {
				lr.skip = true
				return line[:lr.maxLineLength], ErrLineTooLong
			}

			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			return line, err
		} else if err == io.EOF {
			return nil, io.EOF
		} else if !prefix {
			lr.skip = false
		}
	}
}
