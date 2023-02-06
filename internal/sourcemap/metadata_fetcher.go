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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	syncTimeout = 10 * time.Second
)

type MetadataESFetcher struct {
	esClient         *elasticsearch.Client
	index            string
	set              map[identifier]string
	alias            map[identifier]*identifier
	mu               sync.RWMutex
	logger           *logp.Logger
	init             chan struct{}
	invalidationChan chan<- []identifier
}

func NewMetadataFetcher(ctx context.Context, esClient *elasticsearch.Client, index string) (MetadataFetcher, <-chan []identifier) {
	invalidationCh := make(chan []identifier)

	s := &MetadataESFetcher{
		esClient:         esClient,
		index:            index,
		set:              make(map[identifier]string),
		alias:            make(map[identifier]*identifier),
		logger:           logp.NewLogger(logs.Sourcemap),
		init:             make(chan struct{}),
		invalidationChan: invalidationCh,
	}

	s.startBackgroundSync(ctx)

	return s, invalidationCh
}

func (s *MetadataESFetcher) getID(key identifier) (*identifier, bool) {
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

func (s *MetadataESFetcher) ready() <-chan struct{} {
	return s.init
}

func (s *MetadataESFetcher) startBackgroundSync(parent context.Context) {
	go func() {
		s.logger.Debug("populating metadata cache")

		// First run, populate cache
		ctx, cancel := context.WithTimeout(parent, syncTimeout)
		defer cancel()

		if err := s.sync(ctx); err != nil {
			s.logger.Errorf("failed to fetch sourcemaps metadata: %v", err)
		} else {
			// only close the init chan and mark the fetcher as ready if
			// sync succeeded
			close(s.init)
			s.logger.Info("init routine completed")
		}

		go func() {
			// TODO make this a config option ?
			t := time.NewTicker(30 * time.Second)
			defer t.Stop()

			for {
				select {
				case <-t.C:
					ctx, cancel := context.WithTimeout(parent, syncTimeout)

					if err := s.sync(ctx); err != nil {
						s.logger.Errorf("failed to sync sourcemaps metadata: %v", err)
					}

					cancel()
				case <-parent.Done():
					s.logger.Info("update routine done")
					// close invalidation channel
					close(s.invalidationChan)
					return
				}
			}
		}()
	}()
}

func (s *MetadataESFetcher) sync(ctx context.Context) error {
	sourcemaps := make(map[identifier]string)

	result, err := s.initialSearch(ctx, sourcemaps)
	if err != nil {
		return err
	}

	scrollID := result.ScrollID

	if scrollID == "" {
		s.update(ctx, sourcemaps)
		return nil
	}

	for {
		result, err = s.scrollsearch(ctx, scrollID, sourcemaps)
		if err != nil {
			return fmt.Errorf("failed scroll search: %w", err)
		}

		// From the docs: The initial search request and each subsequent scroll
		// request each return a _scroll_id. While the _scroll_id may change between
		// requests, it doesn't always change - in any case, only the most recently
		// received _scroll_id should be used.
		if result.ScrollID != "" {
			scrollID = result.ScrollID
		}

		// Stop if there are no new updates
		if len(result.Hits.Hits) == 0 {
			break
		}
	}

	s.update(ctx, sourcemaps)
	return nil
}

func (s *MetadataESFetcher) update(ctx context.Context, sourcemaps map[identifier]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var invalidation []identifier

	for id, contentHash := range s.set {
		if updatedHash, ok := sourcemaps[id]; ok {
			// already in the cache, remove from the updates.
			delete(sourcemaps, id)

			// content hash changed, invalidate the sourcemap cache and update hash
			if contentHash != updatedHash {
				sourcemaps[id] = updatedHash

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

			// remove aliases
			for _, k := range getAliases(id.name, id.version, id.path) {
				delete(s.alias, k)
			}
		}
	}

	s.invalidationChan <- invalidation

	// add new sourcemaps to the metadata cache.
	for id, contentHash := range sourcemaps {
		s.set[id] = contentHash
		s.logger.Debugf("Added metadata id %v", id)
		// store aliases with a pointer to the original id.
		// The id is then passed over to the backend fetcher
		// to minimize the size of the lru cache and
		// and increase cache hits.
		for _, k := range getAliases(id.name, id.version, id.path) {
			s.logger.Debugf("Added metadata alias %v -> %v", k, id)
			s.alias[k] = &id
		}
	}

	s.logger.Debugf("Metadata cache now has %d entries.", len(s.set))
}

func (s *MetadataESFetcher) initialSearch(ctx context.Context, updates map[identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runSearchQuery(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run initial search query: %w", err)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataESFetcher) runSearchQuery(ctx context.Context) (*esapi.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(queryMetadata()); err != nil {
		return nil, fmt.Errorf("failed to encode metadata query: %w", err)
	}

	req := esapi.SearchRequest{
		Index:          []string{s.index},
		Body:           &buf,
		TrackTotalHits: true,
		Scroll:         time.Minute,
	}
	return req.Do(ctx, s.esClient)
}

func queryMetadata() map[string]interface{} {
	return search(
		sources([]string{"service.*", "file.path", "content_sha256"}),
	)
}

type esSearchSourcemapResponse struct {
	ScrollID string `json:"_scroll_id"`
	esSourcemapResponse
}

func (s *MetadataESFetcher) handleUpdateRequest(resp *esapi.Response, updates map[identifier]string) (*esSearchSourcemapResponse, error) {
	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%w: %s: %s", errFetcherUnvailable, resp.Status(), string(b))
		}
		return nil, fmt.Errorf("ES returned unknown status code: %s", resp.Status())
	}

	// parse response
	body, err := parseResponse(resp.Body, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	for _, v := range body.Hits.Hits {
		id := identifier{
			name:    v.Source.Service.Name,
			version: v.Source.Service.Version,
			path:    v.Source.File.BundleFilepath,
		}

		updates[id] = v.Source.ContentHash
	}

	return body, nil
}

func parseResponse(body io.ReadCloser, logger *logp.Logger) (*esSearchSourcemapResponse, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var esSourcemapResponse esSearchSourcemapResponse
	if err := json.Unmarshal(b, &esSourcemapResponse); err != nil {
		return nil, err
	}

	return &esSourcemapResponse, nil
}

func (s *MetadataESFetcher) scrollsearch(ctx context.Context, scrollID string, updates map[identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runScrollSearchQuery(ctx, scrollID)
	if err != nil {
		return nil, fmt.Errorf("failed to run scroll search query: %w", err)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataESFetcher) runScrollSearchQuery(ctx context.Context, id string) (*esapi.Response, error) {
	req := esapi.ScrollRequest{
		ScrollID: id,
		Scroll:   time.Minute,
	}
	return req.Do(ctx, s.esClient)
}
