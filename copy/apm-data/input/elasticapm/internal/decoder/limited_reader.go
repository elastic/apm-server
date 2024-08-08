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
	"io"

	"github.com/pkg/errors"
)

// LimitedReader is like io.LimitedReader, but returns a
// requestError upon detecting a request that is too large.
//
// Based on net/http.maxBytesReader.
type LimitedReader struct {
	R   io.Reader
	err error
	N   int64
}

// Read implements the standard Read interface, returning an
// error if more than l.N bytes are read.
//
// After each read, l.N is decremented by the number of bytes
// read; if an error is returned due to the l.N limit being
// exceeded, on return l.N will be set to a negative value.
func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.err != nil || len(p) == 0 {
		return 0, l.err
	}
	if int64(len(p)) > l.N+1 {
		p = p[:l.N+1]
	}
	n, err = l.R.Read(p)

	if int64(n) <= l.N {
		l.N -= int64(n)
		l.err = err
		return n, err
	}

	n, l.N = int(l.N), l.N-int64(n)
	l.err = errors.New("too large")
	return n, l.err
}
