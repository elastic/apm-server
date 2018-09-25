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
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const burstMultiplier = 3

// The rlCache is a simple lru cache holding N=size rate limiter entities. Every
// rate limiter entity allows N=rateLimit hits per key (=IP) per second, and has a
// burst queue of limit*5.
// As the used lru cache is of a fixed size, cache entries can get evicted, in
// which case the evicted limiter is reused as the new rate limiter for the
// current key. This adds a certain random factor to the rate limiting in case the
// cache is full. The purpose is to avoid bypassing the rate limiting by sending
// requests from cache_size*2 unique keys, which would lead to evicted keys and
// the creation of new rate limiter entities with full allowance.
type rlCache struct {
	cache *simplelru.LRU
	limit int

	mu             sync.Mutex //guards limiter in cache
	evictedLimiter *rate.Limiter
}

func NewRlCache(size, rateLimit int) (*rlCache, error) {
	if size <= 0 || rateLimit < 0 {
		return nil, errors.New("cache initialization: size and rateLimit must be greater than zero")
	}

	rlc := rlCache{limit: rateLimit}

	var onEvicted = func(_ interface{}, value interface{}) {
		rlc.evictedLimiter = *value.(**rate.Limiter)
	}

	c, err := simplelru.NewLRU(size, simplelru.EvictCallback(onEvicted))
	if err != nil {
		return nil, err
	}
	rlc.cache = c
	return &rlc, nil
}

func (rlc *rlCache) getRateLimiter(key string) *rate.Limiter {
	// fetch the rate limiter from the cache, if a cache is given
	if rlc.cache == nil || rlc.limit == -1 {
		return nil
	}

	// lock get and add action for cache to allow proper eviction handling without
	// race conditions.
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	if l, ok := rlc.cache.Get(key); ok {
		return *l.(**rate.Limiter)
	}

	var limiter *rate.Limiter
	if evicted := rlc.cache.Add(key, &limiter); evicted {
		limiter = rlc.evictedLimiter
	} else {
		limiter = rate.NewLimiter(rate.Limit(rlc.limit), rlc.limit*burstMultiplier)
	}
	return limiter
}
