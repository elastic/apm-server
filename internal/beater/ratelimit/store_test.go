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
	"net/netip"
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
	rlA := store.ForIP(netip.MustParseAddr("127.0.0.1"))
	rlA.AllowN(time.Now(), 3)

	// add new limiter
	rlB := store.ForIP(netip.MustParseAddr("127.0.0.2"))
	rlB.AllowN(time.Now(), 2)

	// reuse evicted limiter rlA
	rlC := store.ForIP(netip.MustParseAddr("127.0.0.3"))
	assert.False(t, rlC.Allow())
	assert.Equal(t, rlC, store.evictedLimiter)

	// reuse evicted limiter rlB
	rlD := store.ForIP(netip.MustParseAddr("127.0.0.1"))
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
	limiter := store.ForIP(netip.MustParseAddr("127.0.0.1"))
	assert.NotNil(t, limiter)
}

// BenchmarkStoreMemory to estimate the rate limiter size of N IP address
// Run with: go test -v -race -bench=BenchmarkStoreMemory -benchmem | benchstat -
// Output:
/*
	goos: darwin
	goarch: arm64
	pkg: github.com/elastic/apm-server/internal/beater/ratelimit
	cpu: Apple M4 Pro
													│      -       │
													│    sec/op    │
	StoreMemory/1_ip/empty_store-14                   269.8n ± ∞ ¹
	StoreMemory/1_ip/populated_store-14               970.9n ± ∞ ¹
	StoreMemory/1000_ips_default/empty_store-14       13.54µ ± ∞ ¹
	StoreMemory/1000_ips_default/populated_store-14   774.0µ ± ∞ ¹
	StoreMemory/5000_ips/empty_store-14               52.31µ ± ∞ ¹
	StoreMemory/5000_ips/populated_store-14           3.984m ± ∞ ¹
	StoreMemory/10000_ips/empty_store-14              108.7µ ± ∞ ¹
	StoreMemory/10000_ips/populated_store-14          8.080m ± ∞ ¹
	geomean                                           68.81µ
	¹ need >= 6 samples for confidence interval at level 0.95

													│       -       │
													│     B/op      │
	StoreMemory/1_ip/empty_store-14                     320.0 ± ∞ ¹
	StoreMemory/1_ip/populated_store-14                 408.0 ± ∞ ¹
	StoreMemory/1000_ips_default/empty_store-14       68.23Ki ± ∞ ¹
	StoreMemory/1000_ips_default/populated_store-14   154.2Ki ± ∞ ¹
	StoreMemory/5000_ips/empty_store-14               340.2Ki ± ∞ ¹
	StoreMemory/5000_ips/populated_store-14           769.9Ki ± ∞ ¹
	StoreMemory/10000_ips/empty_store-14              672.2Ki ± ∞ ¹
	StoreMemory/10000_ips/populated_store-14          1.496Mi ± ∞ ¹
	geomean                                           65.84Ki
	¹ need >= 6 samples for confidence interval at level 0.95

													│      -       │
													│  allocs/op   │
	StoreMemory/1_ip/empty_store-14                    5.000 ± ∞ ¹
	StoreMemory/1_ip/populated_store-14                7.000 ± ∞ ¹
	StoreMemory/1000_ips_default/empty_store-14        5.000 ± ∞ ¹
	StoreMemory/1000_ips_default/populated_store-14   2.005k ± ∞ ¹
	StoreMemory/5000_ips/empty_store-14                5.000 ± ∞ ¹
	StoreMemory/5000_ips/populated_store-14           10.01k ± ∞ ¹
	StoreMemory/10000_ips/empty_store-14               5.000 ± ∞ ¹
	StoreMemory/10000_ips/populated_store-14          20.00k ± ∞ ¹
	geomean                                            80.46
*/
func BenchmarkStoreMemory(b *testing.B) {
	cases := []struct {
		name string
		size int
	}{
		{
			name: "1_ip",
			size: 1,
		},
		{
			name: "1000_ips_default",
			size: 1000,
		},
		{
			name: "5000_ips",
			size: 5000,
		},
		{
			name: "10000_ips",
			size: 10000,
		},
	}

	ips := make([]netip.Addr, 10000)
	for i := range ips {
		ips[i] = netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})
	}

	for _, tc := range cases {
		b.Run(tc.name+"/empty_store", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = NewStore(tc.size, 300, 3)
			}
		})

		b.Run(tc.name+"/populated_store", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				s, _ := NewStore(tc.size, 300, 3)
				for i := 0; i < tc.size; i++ {
					_ = s.ForIP(ips[i])
				}
			}
		})
	}
}
