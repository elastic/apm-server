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
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// Manager interface defines a method to retrieve a rate.Limiter for a key
type Manager interface {
	Acquire(string) (*rate.Limiter, bool)
}

// LRUCache is a simple lru cache holding N=size rate limiter entities. Every
// rate limiter entity allows N=rateLimit hits per key (=IP) per second, and has a
// burst queue of limit*5.
// As the used lru cache is of a fixed size, cache entries can get evicted, in
// which case the evicted limiter is reused as the new rate limiter for the
// current key. This adds a certain random factor to the rate limiting in case the
// cache is full. The purpose is to avoid bypassing the rate limiting by sending
// requests from cache_size*2 unique keys, which would lead to evicted keys and
// the creation of new rate limiter entities with full allowance.
type LRUCache struct {
	cache          *simplelru.LRU
	limit          int
	burstFactor    int
	mu             sync.Mutex //guards limiter in cache
	evictedLimiter *rate.Limiter
}

// NewLRUCache returns a new instance of the LRUCache
func NewLRUCache(size, rateLimit, burstFactor int) (*LRUCache, error) {
	if size <= 0 || rateLimit < 0 {
		return nil, errors.New("cache initialization: size and rateLimit must be greater than zero")
	}

	lru := LRUCache{limit: rateLimit, burstFactor: burstFactor}

	var onEvicted = func(_ interface{}, value interface{}) {
		lru.evictedLimiter = *value.(**rate.Limiter)
	}

	c, err := simplelru.NewLRU(size, simplelru.EvictCallback(onEvicted))
	if err != nil {
		return nil, err
	}
	lru.cache = c
	return &lru, nil
}

// Acquire returns a rate.Limiter instance for the given key
func (lru *LRUCache) Acquire(key string) (*rate.Limiter, bool) {
	// fetch the rate limiter from the cache, if a cache is given
	if lru == nil || lru.cache == nil || lru.limit == -1 {
		return nil, false
	}

	// lock get and add action for cache to allow proper eviction handling without
	// race conditions.
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if l, ok := lru.cache.Get(key); ok {
		return *l.(**rate.Limiter), true
	}

	var limiter *rate.Limiter
	if evicted := lru.cache.Add(key, &limiter); evicted {
		limiter = lru.evictedLimiter
	} else {
		limiter = rate.NewLimiter(rate.Limit(lru.limit), lru.limit*lru.burstFactor)
	}
	return limiter, true
}
