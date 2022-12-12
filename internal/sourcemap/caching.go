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
	"math"
	"strings"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	minCleanupIntervalSeconds float64 = 60
)

var (
	errMsgFailure = "failure querying"
	errInit       = errors.New("Cache cannot be initialized. Expiration and CleanupInterval need to be >= 0")
)

// Fetcher is an interface for fetching a source map with a given service name, service version,
// and bundle filepath.
type Fetcher interface {
	// Fetch fetches a source map with a given service name, service version, and bundle filepath.
	//
	// If there is no such source map available, Fetch returns a nil Consumer.
	Fetch(ctx context.Context, serviceName, serviceVersion, bundleFilepath string) (*sourcemap.Consumer, error)
}

// CachingFetcher wraps a Fetcher, caching source maps in memory and fetching from the wrapped Fetcher on cache misses.
type CachingFetcher struct {
	cache   *lru.Cache
	backend Fetcher
	logger  *logp.Logger
}

// NewCachingFetcher returns a CachingFetcher that wraps backend, caching results for the configured cacheExpiration.
func NewCachingFetcher(
	backend Fetcher,
	invalidationChan <-chan Identifier,
	cacheSize int,
) (*CachingFetcher, error) {
	c, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	go func() {
		for i := range invalidationChan {
			key := cacheKey([]string{i.name, i.version, i.path})
			c.Remove(key)
		}
	}()

	return &CachingFetcher{
		cache:   c,
		backend: backend,
		logger:  logp.NewLogger(logs.Sourcemap),
	}, nil
}

// Fetch fetches a source map from the cache or wrapped backend.
func (s *CachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	key := cacheKey([]string{name, version, path})

	// fetch from cache
	if val, found := s.cache.Get(key); found {
		consumer, _ := val.(*sourcemap.Consumer)
		return consumer, nil
	}

	// fetch from the store and ensure caching for all non-temporary results
	consumer, err := s.backend.Fetch(ctx, name, version, path)
	if err != nil {
		if !strings.Contains(err.Error(), errMsgFailure) {
			s.add(key, nil)
		}
		return nil, err
	}
	s.add(key, consumer)
	return consumer, nil
}

func (s *CachingFetcher) add(key string, consumer *sourcemap.Consumer) {
	s.cache.Add(key, consumer)
	if !s.logger.IsDebug() {
		return
	}
	s.logger.Debugf("Added id %v. Cache now has %v entries.", key, s.cache.Len())
}

func cacheKey(s []string) string {
	return strings.Join(s, "_")
}

func cleanupInterval(ttl time.Duration) time.Duration {
	return time.Duration(math.Max(ttl.Seconds(), minCleanupIntervalSeconds)) * time.Second
}

func parseSourceMap(data string) (*sourcemap.Consumer, error) {
	if data == "" {
		return nil, nil
	}
	consumer, err := sourcemap.Parse("", []byte(data))
	if err != nil {
		return nil, errors.Wrap(err, errMsgParseSourcemap)
	}
	return consumer, nil
}
