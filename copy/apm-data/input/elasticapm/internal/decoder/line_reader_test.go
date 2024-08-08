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
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
)

func TestLineReaderEmpty(t *testing.T) {
	readBuf := bytes.NewBufferString("")
	lr := NewLineReader(bufio.NewReaderSize(readBuf, 10), 10)

	buf, err := lr.ReadLine()
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []byte{}, buf)
}

func TestLineReader(t *testing.T) {
	for _, r := range [](func(io.Reader) io.Reader){
		func(r io.Reader) io.Reader { return r },
		iotest.HalfReader,
		iotest.OneByteReader,
		iotest.DataErrReader,
	} {
		readBuf := bytes.NewBufferString("line1\nline2\n01234567890123456789\nline4\nline5")
		lr := NewLineReader(bufio.NewReaderSize(r(readBuf), 10), 10)

		buf, err := lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line1"), buf)

		buf, err = lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line2"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, "0123456789", string(buf))

		buf, err = lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line4"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, []byte("line5"), buf)

	}
}

func TestLineReaderEndWithNewline(t *testing.T) {
	for _, r := range [](func(io.Reader) io.Reader){
		func(r io.Reader) io.Reader { return r },
		iotest.HalfReader,
		iotest.OneByteReader,
		iotest.DataErrReader,
	} {
		readBuf := bytes.NewBufferString("line1\nline2\n01234567890123456789\nline4\nline5\n")
		lr := NewLineReader(bufio.NewReaderSize(r(readBuf), 10), 10)

		buf, err := lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line1"), buf)

		buf, err = lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line2"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, "0123456789", string(buf))

		buf, err = lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line4"), buf)

		buf, err = lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line5"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, []byte{}, buf)
	}
}

func TestLineReaderTwoLongLines(t *testing.T) {
	for _, r := range [](func(io.Reader) io.Reader){
		func(r io.Reader) io.Reader { return r },
		iotest.HalfReader,
		iotest.OneByteReader,
		iotest.DataErrReader,
	} {
		readBuf := bytes.NewBufferString("line1\nlong-string-with-no-newlines-at-all\nanother-long-string-with-no-newlines-at-all\nline2")
		lr := NewLineReader(bufio.NewReaderSize(r(readBuf), 10), 10)

		buf, err := lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line1"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, []byte("long-strin"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, []byte("another-lo"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, []byte("line2"), buf)
	}
}

func TestLineReaderNoLines(t *testing.T) {
	for _, r := range [](func(io.Reader) io.Reader){
		func(r io.Reader) io.Reader { return r },
		iotest.HalfReader,
		iotest.OneByteReader,
		iotest.DataErrReader,
	} {
		readBuf := bytes.NewBufferString("line1\nlong-string-with-no-newlines-at-all\nanother-long-string-with-no-newlines-at-all")
		lr := NewLineReader(bufio.NewReaderSize(r(readBuf), 10), 10)

		buf, err := lr.ReadLine()
		assert.NoError(t, err)
		assert.Equal(t, []byte("line1"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, []byte("long-strin"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, ErrLineTooLong, err)
		assert.Equal(t, []byte("another-lo"), buf)

		buf, err = lr.ReadLine()
		assert.Equal(t, io.EOF, err)
		assert.Nil(t, buf)
	}
}

func DeadlineReader(reader io.Reader) io.Reader { return &deadlineReader{reader, 0} }

type deadlineReader struct {
	r     io.Reader
	count int
}

func (r *deadlineReader) Read(p []byte) (int, error) {
	r.count++
	if r.count >= 2 {
		return 0, iotest.ErrTimeout
	}
	return r.r.Read(p)
}

func TestLineReaderDeadline(t *testing.T) {
	readBuf := bytes.NewBufferString("line1\nline2\nlong-string-with-no-newlines-at-all")
	reader := DeadlineReader(readBuf)

	lr := NewLineReader(bufio.NewReaderSize(reader, 10), 10)

	buf, err := lr.ReadLine()
	assert.NoError(t, err)
	assert.Equal(t, []byte("line1"), buf)

	buf, err = lr.ReadLine()
	assert.NoError(t, err)
	assert.Equal(t, []byte("line2"), buf)

	buf, err = lr.ReadLine()
	assert.Equal(t, iotest.ErrTimeout, err)
	assert.Equal(t, []byte("long"), buf)

	buf, err = lr.ReadLine()
	assert.Equal(t, iotest.ErrTimeout, err)
	assert.Equal(t, []byte{}, buf)
}

func TestLineReaderShort(t *testing.T) {
	readBuf := bytes.NewBufferString("line1\nline2")

	lr := NewLineReader(bufio.NewReaderSize(readBuf, 1024), 1024)

	buf, err := lr.ReadLine()
	assert.NoError(t, err)
	assert.Equal(t, []byte("line1"), buf)

	buf, err = lr.ReadLine()
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []byte("line2"), buf)
}
