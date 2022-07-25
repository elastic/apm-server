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
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// Store is a LRU cache holding cache_size rate limiters,
// allowing N hits per cache key.
// Evicted rate limiters are reused for the current key.
// This adds a random factor to the rate limiting if the cache is full.
// The purpose is to avoid bypassing the rate limiting by sending
// requests from cache_size*2 unique keys, which would lead to
// the creation of new rate limiter entities with full allowance.
type Store struct {
	cache          simplelru.LRU
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
	store.cache = *c
	return &store, nil
}

// ForIP returns a rate limiter for the given IP.
func (s *Store) ForIP(ip net.IP) *rate.Limiter {
	key := ip.String()

	// lock get and add action for cache to allow proper eviction handling without
	// race conditions.
	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.cache.Get(key); ok {
		return *l.(**rate.Limiter)
	}

	var limiter *rate.Limiter
	if evicted := s.cache.Add(key, &limiter); evicted {
		limiter = s.evictedLimiter
	} else {
		limiter = rate.NewLimiter(rate.Limit(s.limit), s.limit*s.burstFactor)
	}
	return limiter
}
