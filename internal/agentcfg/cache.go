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

package agentcfg

import (
	"hash/maphash"
	"time"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-freelru"
)

type cache struct {
	logger  *logp.Logger
	gocache *freelru.ShardedLRU[cacheKey, Result]
}

func newCache(logger *logp.Logger, exp time.Duration) (*cache, error) {
	logger.Infof("Cache creation with expiration %v.", exp)
	seed := maphash.MakeSeed()
	hashFunc := func(key cacheKey) uint32 {
		return uint32(maphash.Comparable[cacheKey](seed, key))
	}
	lru, err := freelru.NewSharded[cacheKey, Result](8192, hashFunc)
	if err != nil {
		return nil, err
	}
	lru.SetLifetime(exp)

	return &cache{
		logger:  logger,
		gocache: lru,
	}, nil
}

func (c *cache) fetch(query Query, fetch func() (Result, error)) (Result, error) {
	// return from cache if possible
	value, found := c.gocache.Get(query.id())
	if found {
		return value, nil
	}
	// retrieve resource from external source
	result, err := fetch()
	if err != nil {
		return result, err
	}
	c.gocache.Add(query.id(), result)

	if c.logger.IsDebug() {
		c.logger.Debugf("Cache size %v. Added ID %v.", c.gocache.Len(), query.id())
	}
	return result, nil
}
