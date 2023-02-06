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

package sourcemap

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sourcemap/sourcemap"
	lru "github.com/hashicorp/golang-lru"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

// BodyCachingFetcher wraps a Fetcher, caching source maps in memory and fetching from the wrapped Fetcher on cache misses.
type BodyCachingFetcher struct {
	cache   *lru.Cache
	backend Fetcher
	logger  *logp.Logger
}

// NewBodyCachingFetcher returns a CachingFetcher that wraps backend, caching results for the configured cacheExpiration.
func NewBodyCachingFetcher(
	backend Fetcher,
	cacheSize int,
	invalidationChan <-chan []identifier,
) (*BodyCachingFetcher, error) {
	logger := logp.NewLogger(logs.Sourcemap)

	lruCache, err := lru.NewWithEvict(cacheSize, func(key, value interface{}) {
		logger.Debugf("Removed id %v", key)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create lru cache for caching fetcher: %w", err)
	}

	go func() {
		logger.Debug("listening for invalidation...")

		for arr := range invalidationChan {
			for _, id := range arr {
				logger.Debugf("Invalidating id %v", id)
				lruCache.Remove(id)
			}
		}
	}()

	return &BodyCachingFetcher{
		cache:   lruCache,
		backend: backend,
		logger:  logger,
	}, nil
}

// Fetch fetches a source map from the cache or wrapped backend.
func (s *BodyCachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	key := identifier{
		name:    name,
		version: version,
		path:    path,
	}

	// fetch from cache
	if val, found := s.cache.Get(key); found {
		consumer, _ := val.(*sourcemap.Consumer)
		return consumer, nil
	}

	// fetch from the store and ensure caching for all non-temporary results
	consumer, err := s.backend.Fetch(ctx, name, version, path)
	if err != nil {
		if errors.Is(err, errMalformedSourcemap) {
			s.add(key, nil)
		}
		return nil, err
	}
	s.add(key, consumer)
	return consumer, nil
}

func (s *BodyCachingFetcher) add(key identifier, consumer *sourcemap.Consumer) {
	s.cache.Add(key, consumer)
	s.logger.Debugf("Added id %v. Cache now has %v entries.", key, s.cache.Len())
}
