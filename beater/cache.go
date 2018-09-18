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

	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

const burstMultiplier = 5

var limiterPool sync.Pool

type rlCache struct {
	cache *lru.Cache
	limit int
	mutex sync.Mutex //guards limiter in cache
}

func NewRlCache(size, rateLimit int) (*rlCache, error) {
	if size <= 0 || rateLimit < 0 {
		return nil, errors.New("Cache initialization went wrong.")
	}

	var onEvicted = func(key interface{}, value interface{}) {
		limiterPool.Put(*value.(**rate.Limiter))
	}

	c, err := lru.NewWithEvict(size, onEvicted)
	if err != nil {
		return nil, err
	}
	rlc := rlCache{cache: c, limit: rateLimit}
	return &rlc, nil
}

func (rlc *rlCache) getRateLimiter(key string) *rate.Limiter {
	if rlc.cache == nil || rlc.limit == -1 {
		return nil
	}

	// fetch the rate limiter from the cache, if a cache is given
	getLimiter := func() (*rate.Limiter, bool) {
		if l, ok := rlc.cache.Get(key); ok {
			return *l.(**rate.Limiter), true
		}
		return nil, false
	}

	limiter, ok := getLimiter()
	if ok {
		return limiter
	}

	rlc.mutex.Lock()
	defer rlc.mutex.Unlock()
	limiter, ok = getLimiter()
	if !ok {
		if evicted := rlc.cache.Add(key, &limiter); evicted {
			limiter = limiterPool.Get().(*rate.Limiter)
		} else {
			limiter = rate.NewLimiter(rate.Limit(rlc.limit), rlc.limit*burstMultiplier)
		}
	}
	return limiter
}
