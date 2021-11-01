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
	"sync"
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/beats/v7/libbeat/logp"
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
	cache   *gocache.Cache
	backend Fetcher
	logger  *logp.Logger

	mu       sync.Mutex
	inflight map[string]chan struct{}
}

// NewCachingFetcher returns a CachingFetcher that wraps backend, caching results for the configured cacheExpiration.
func NewCachingFetcher(
	backend Fetcher,
	cacheExpiration time.Duration,
) (*CachingFetcher, error) {
	if cacheExpiration < 0 {
		return nil, errInit
	}
	return &CachingFetcher{
		cache:    gocache.New(cacheExpiration, cleanupInterval(cacheExpiration)),
		backend:  backend,
		logger:   logp.NewLogger(logs.Sourcemap),
		inflight: make(map[string]chan struct{}),
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

	// if the value hasn't been found, check to see if there's an inflight
	// request to update the value.
	s.mu.Lock()
	wait, ok := s.inflight[key]
	if ok {
		// found an inflight request, wait for it to complete.
		s.mu.Unlock()

		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// Try to read the value again
		return s.Fetch(ctx, name, version, path)
	}

	// no inflight request found, add a channel to the map and then
	// make the fetch request.
	wait = make(chan struct{})
	s.inflight[key] = wait

	s.mu.Unlock()

	// Once the fetch request is complete, close and remove the channel
	// from the syncronization map.
	defer func() {
		s.mu.Lock()
		delete(s.inflight, key)
		close(wait)
		s.mu.Unlock()
	}()

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
	s.cache.SetDefault(key, consumer)
	if !s.logger.IsDebug() {
		return
	}
	s.logger.Debugf("Added id %v. Cache now has %v entries.", key, s.cache.ItemCount())
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
