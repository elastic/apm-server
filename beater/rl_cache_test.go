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

package beater

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheInitFails(t *testing.T) {
	for _, test := range []struct {
		size  int
		limit int
	}{
		{-1, 1},
		{0, 1},
		{1, -1},
	} {
		c, err := NewRlCache(test.size, test.limit)
		assert.Error(t, err)
		assert.Nil(t, c)
	}
}

func TestCacheEviction(t *testing.T) {
	cache_size := 2
	limit := 1 //multiplied times burstMultiplier 5

	rlc, err := NewRlCache(cache_size, limit)
	require.NoError(t, err)

	// add new limiter
	rl_a := rlc.getRateLimiter("a")
	rl_a.AllowN(time.Now(), 3)

	// add new limiter
	rl_b := rlc.getRateLimiter("b")
	rl_b.AllowN(time.Now(), 2)

	// reuse evicted limiter rl_a
	rl_c := rlc.getRateLimiter("c")
	assert.False(t, rl_c.Allow())
	assert.Equal(t, rl_c, rlc.evictedLimiter)

	// reuse evicted limiter rl_b
	rl_d := rlc.getRateLimiter("a")
	assert.True(t, rl_d.Allow())
	assert.False(t, rl_d.Allow())
	assert.Equal(t, rl_d, rlc.evictedLimiter)
	// check that limiter are independent
	assert.True(t, rl_d != rl_c)
	rlc.evictedLimiter = nil
	assert.NotNil(t, rl_d)
	assert.NotNil(t, rl_c)
}
