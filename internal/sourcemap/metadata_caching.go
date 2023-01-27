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
	"fmt"
	"net/url"
	"sync"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

type MetadataCachingFetcher struct {
	set            map[Identifier]string
	alias          map[Identifier]*Identifier
	mu             sync.RWMutex
	backend        Fetcher
	logger         *logp.Logger
	init           chan struct{}
	updateChan     <-chan map[Identifier]string
	invalidateChan chan<- []Identifier
}

func NewMetadataCachingFetcher(backend Fetcher, in <-chan map[Identifier]string, out chan<- []Identifier) *MetadataCachingFetcher {
	s := &MetadataCachingFetcher{
		set:            make(map[Identifier]string),
		alias:          make(map[Identifier]*Identifier),
		backend:        backend,
		logger:         logp.NewLogger(logs.Sourcemap),
		init:           make(chan struct{}),
		updateChan:     in,
		invalidateChan: out,
	}

	go s.handleUpdates()

	return s
}

func (s *MetadataCachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	original := Identifier{name: name, version: version, path: path}

	select {
	case <-s.init:
		// the mutex is shared by the update goroutine, we need to release it
		// as soon as possible to avoid blocking updates.
		if i, ok := s.getID(original); ok {
			// Only fetch from ES if the sourcemap id exists
			return s.fetch(ctx, i)
		}
	default:
		s.logger.Debugf("Metadata cache not populated. Falling back to backend fetcher for id: %s, %s, %s", name, version, path)
		// init is in progress, ignore the metadata cache and fetch the sourcemap directly
		// return if we get a valid sourcemap or an error
		if c, err := s.backend.Fetch(ctx, original.name, original.version, original.path); c != nil || err != nil {
			return c, err
		}

		s.logger.Debug("Blocking until init is completed")

		// Aliases are only available after init is completed.
		select {
		case <-s.init:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		// first map lookup  will fail but this is not going
		// to be performance issue since it only happens if init
		// is in progress.
		if i, ok := s.getID(original); ok {
			// Only fetch from ES if the sourcemap id exists
			return s.fetch(ctx, i)
		}
	}

	if urlPath, err := url.Parse(path); err == nil {
		// The sourcemap coule be stored in ES with a relative
		// bundle filepath but the request came in with an
		// absolute path
		original.path = urlPath.Path
		if urlPath.Path != path {
			// The sourcemap could be stored on ES under a certain host
			// but a request came in from a different host.
			// Look for an alias to the url path to retrieve the correct
			// host and fetch the sourcemap
			if i, ok := s.getID(original); ok {
				return s.fetch(ctx, i)
			}
		}

		// Clean the url and try again if the result is different from
		// the original bundle filepath
		urlPath.RawQuery = ""
		urlPath.Fragment = ""
		urlPath = urlPath.JoinPath()
		cleanPath := urlPath.String()

		if cleanPath != path {
			s.logger.Debugf("original filepath %s converted to %s", path, cleanPath)
			return s.Fetch(ctx, name, version, cleanPath)
		}
	}

	return nil, fmt.Errorf("unable to find sourcemap.url for service.name=%s service.version=%s bundle.path=%s", name, version, path)
}

func (s *MetadataCachingFetcher) getID(key Identifier) (*Identifier, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.set[key]; ok {
		return &key, ok
	}

	// path is missing from the metadata cache (and ES).
	// Is it an alias ?
	// Try to retrieve the sourcemap from the alias map
	i, ok := s.alias[key]
	return i, ok
}

func (s *MetadataCachingFetcher) fetch(ctx context.Context, key *Identifier) (*sourcemap.Consumer, error) {
	c, err := s.backend.Fetch(ctx, key.name, key.version, key.path)

	// log a message if the sourcemap is present in the cache but the backend fetcher did not
	// find it.
	if err == nil && c == nil {
		return nil, fmt.Errorf("unable to find sourcemap for service.name=%s service.version=%s bundle.path=%s", key.name, key.version, key.path)
	}

	return c, err
}

func (s *MetadataCachingFetcher) handleUpdates() {
	// run once for init
	s.update(<-s.updateChan)
	close(s.init)

	// wait for updates
	for updates := range s.updateChan {
		s.update(updates)
	}
	close(s.invalidateChan)
}

func (s *MetadataCachingFetcher) update(updates map[Identifier]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var invalidation []Identifier

	for id, contentHash := range s.set {
		if updatedHash, ok := updates[id]; ok {
			// already in the cache, remove from the updates.
			delete(updates, id)

			// content hash changed, invalidate the sourcemap cache
			if contentHash != updatedHash {
				s.logger.Debugf("Hash changed: %s -> %s: invalidating %v", contentHash, updatedHash, id)
				invalidation = append(invalidation, id)
			}
		} else {
			// the sourcemap no longer exists in ES.
			// invalidate the sourcemap cache.
			invalidation = append(invalidation, id)

			// the sourcemap no longer exists in ES.
			// remove from metadata cache
			delete(s.set, id)

			// remove alias
			for _, k := range GetAliases(id.name, id.version, id.path) {
				delete(s.alias, k)
			}
		}
	}

	s.invalidateChan <- invalidation

	// add new sourcemaps to the metadata cache.
	for id, contentHash := range updates {
		s.set[id] = contentHash
		s.logger.Debugf("Added metadata id %v", id)
		// store aliases with a pointer to the original id.
		// The id is then passed over to the backend fetcher
		// to minimize the size of the lru cache and
		// and increase cache hits.
		for _, k := range GetAliases(id.name, id.version, id.path) {
			s.logger.Debugf("Added metadata alias %v -> %v", k, id)
			s.alias[k] = &id
		}
	}

	s.logger.Debugf("Metadata cache now has %d entries.", len(s.set))
}
