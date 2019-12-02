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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitedReaderRead(t *testing.T) {
	test := func(t *testing.T, buflen, expectedLen int) {
		r := &LimitedReader{R: strings.NewReader("abc"), N: 3}
		out := make([]byte, buflen)
		n, err := r.Read(out)
		require.NoError(t, err)
		assert.Equal(t, expectedLen, n)
		assert.Equal(t, "abc"[:n], string(out[:n]))
		assert.Equal(t, int64(3-n), r.N)
	}
	test(t, 1, 1)
	test(t, 2, 2)
	test(t, 3, 3)
	test(t, 4, 3)
}

func TestLimitedReaderLimited(t *testing.T) {
	r := &LimitedReader{R: strings.NewReader("abcd"), N: 3}
	out := make([]byte, 4)
	n, err := r.Read(out)
	require.Error(t, err)
	require.EqualError(t, err, "too large")
	assert.Equal(t, 3, n)
	assert.Equal(t, "abc", string(out[:n]))
	assert.Equal(t, int64(-1), r.N)
}
