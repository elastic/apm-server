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
	"net/http"
	"sync"

	"github.com/elastic/apm-server/utility"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// Store is a simple lru cache holding N=size rate limiter entities. Every
// rate limiter entity allows N=rateLimit hits per key (=IP) per second, and has a
// burst queue of limit*5.
// As the used lru cache is of a fixed size, cache entries can get evicted, in
// which case the evicted limiter is reused as the new rate limiter for the
// current key. This adds a certain random factor to the rate limiting in case the
// cache is full. The purpose is to avoid bypassing the rate limiting by sending
// requests from cache_size*2 unique keys, which would lead to evicted keys and
// the creation of new rate limiter entities with full allowance.
type Store struct {
	cache          *simplelru.LRU
	limit          int
	burstFactor    int
	mu             sync.Mutex //guards limiter in cache
	evictedLimiter *rate.Limiter
}

// NewStore returns a new instance of the Store
func NewStore(size, rateLimit, burstFactor int) (*Store, error) {
	if size <= 0 || rateLimit < 0 {
		return nil, errors.New("cache initialization: size must be greater than zero")
	}

	store := Store{limit: rateLimit, burstFactor: burstFactor}

	var onEvicted = func(_ interface{}, value interface{}) {
		store.evictedLimiter = *value.(**rate.Limiter)
	}

	c, err := simplelru.NewLRU(size, simplelru.EvictCallback(onEvicted))
	if err != nil {
		return nil, err
	}
	store.cache = c
	return &store, nil
}

// acquire returns a rate.Limiter instance for the given key
func (s *Store) acquire(key string) (*rate.Limiter, bool) {
	// fetch the rate limiter from the cache, if a cache is given
	if s == nil || s.cache == nil {
		return nil, false
	}

	// lock get and add action for cache to allow proper eviction handling without
	// race conditions.
	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.cache.Get(key); ok {
		return *l.(**rate.Limiter), true
	}

	var limiter *rate.Limiter
	if evicted := s.cache.Add(key, &limiter); evicted {
		limiter = s.evictedLimiter
	} else {
		limiter = rate.NewLimiter(rate.Limit(s.limit), s.limit*s.burstFactor)
	}
	return limiter, true
}

// PerIP returns a rate limiter per request IP
func (s *Store) PerIP(r *http.Request) (*rate.Limiter, bool) {
	return s.acquire(utility.RemoteAddr(r))
}
