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

package ratelimit

import (
	"net"
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
		c, err := NewStore(test.size, test.limit, 3)
		assert.Error(t, err)
		assert.Nil(t, c)
	}
}

func TestCacheEviction(t *testing.T) {
	cacheSize := 2
	limit := 1 //multiplied times BurstMultiplier 3

	store, err := NewStore(cacheSize, limit, 3)
	require.NoError(t, err)

	// add new limiter
	rlA := store.ForIP(net.ParseIP("127.0.0.1"))
	rlA.AllowN(time.Now(), 3)

	// add new limiter
	rlB := store.ForIP(net.ParseIP("127.0.0.2"))
	rlB.AllowN(time.Now(), 2)

	// reuse evicted limiter rlA
	rlC := store.ForIP(net.ParseIP("127.0.0.3"))
	assert.False(t, rlC.Allow())
	assert.Equal(t, rlC, store.evictedLimiter)

	// reuse evicted limiter rlB
	rlD := store.ForIP(net.ParseIP("127.0.0.1"))
	assert.True(t, rlD.Allow())
	assert.False(t, rlD.Allow())
	assert.Equal(t, rlD, store.evictedLimiter)
	// check that limiter are independent
	assert.True(t, rlD != rlC)
	store.evictedLimiter = nil
	assert.NotNil(t, rlD)
	assert.NotNil(t, rlC)
}

func TestCacheOk(t *testing.T) {
	store, err := NewStore(1, 1, 1)
	require.NoError(t, err)
	limiter := store.ForIP(net.ParseIP("127.0.0.1"))
	assert.NotNil(t, limiter)
}
