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
	"net/url"
	"sync"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type MetadataCachingFetcher struct {
	esClient         *elasticsearch.Client
	set              map[Identifier]string
	alias            map[Identifier]*Identifier
	mu               sync.RWMutex
	backend          Fetcher
	logger           *logp.Logger
	index            string
	init             chan struct{}
	invalidationChan chan<- Identifier
}

func NewMetadataCachingFetcher(
	c *elasticsearch.Client,
	backend Fetcher,
	index string,
	invalidationChan chan<- Identifier,
) *MetadataCachingFetcher {
	return &MetadataCachingFetcher{
		esClient:         c,
		index:            index,
		set:              make(map[Identifier]string),
		alias:            make(map[Identifier]*Identifier),
		backend:          backend,
		logger:           logp.NewLogger(logs.Sourcemap),
		init:             make(chan struct{}),
		invalidationChan: invalidationChan,
	}
}

func (s *MetadataCachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	original := Identifier{name: name, version: version, path: path}

	select {
	case <-s.init:
		// the mutex is shared by the update goroutine, we need to release it
		// as soon as possible to avoid blocking updates.
		if s.hasID(original) {
			// Only fetch from ES if the sourcemap id exists
			return s.fetch(ctx, &original)
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
		<-s.init
	}

	// path is missing from the metadata cache (and ES).
	// Is it an alias ?
	// Try to retrieve the sourcemap from the alias map
	// Only fetch from ES if the sourcemap alias exists
	if id, found := s.getAlias(original); found {
		return s.fetch(ctx, id)
	}

	if urlPath, err := url.Parse(path); err == nil {
		// The sourcemap coule be stored in ES with a relative
		// bundle filepath but the request came in with an
		// absolute path
		original.path = urlPath.Path
		if urlPath.Path != path && s.hasID(original) {
			return s.fetch(ctx, &original)
		}

		// The sourcemap could be stored on ES under a certain host
		// but a request came in from a different host.
		// Look for an alias to the url path to retrieve the correct
		// host and fetch the sourcemap
		if id, found := s.getAlias(original); found {
			return s.fetch(ctx, id)
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

	return nil, nil
}

func (s *MetadataCachingFetcher) hasID(key Identifier) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.set[key]
	return ok
}

func (s *MetadataCachingFetcher) getAlias(key Identifier) (*Identifier, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	i, ok := s.alias[key]
	return i, ok
}

func (s *MetadataCachingFetcher) fetch(ctx context.Context, key *Identifier) (*sourcemap.Consumer, error) {
	c, err := s.backend.Fetch(ctx, key.name, key.version, key.path)

	// log a message if the sourcemap is present in the cache but the backend fetcher did not
	// find it.
	if err == nil && c == nil {
		s.logger.Debugf("Backend fetcher failed to retrieve sourcemap: %v", key)
	}

	return c, err
}

func (s *MetadataCachingFetcher) update(ctx context.Context, updates map[Identifier]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, contentHash := range s.set {
		if updatedHash, ok := updates[id]; ok {
			// already in the cache, remove from the updates.
			delete(updates, id)

			// content hash changed, invalidate the sourcemap cache
			if contentHash != updatedHash {
				s.logger.Debugf("Hash changed: %s -> %s: invalidating %v", contentHash, updatedHash, id)
				select {
				case s.invalidationChan <- id:
				case <-ctx.Done():
					s.logger.Errorf("ctx finished while invalidating id: %v", ctx.Err())
					return
				}

			}
		} else {
			// the sourcemap no longer exists in ES.
			// invalidate the sourcemap cache.
			select {
			case s.invalidationChan <- id:
			case <-ctx.Done():
				s.logger.Errorf("ctx finished while invaliding id: %v", ctx.Err())
				return
			}

			// remove from metadata cache
			delete(s.set, id)
			// remove alias
			for _, k := range GetAliases(id.name, id.version, id.path) {
				delete(s.alias, k)
			}
		}
	}
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

func (s *MetadataCachingFetcher) StartBackgroundSync(parent context.Context) {
	go func() {
		// First run, populate cache
		ctx, cleanup := context.WithTimeout(parent, 10*time.Second)
		defer cleanup()

		defer close(s.init)

		if err := s.sync(ctx); err != nil {
			s.logger.Errorf("failed to fetch sourcemaps metadata: %v", err)
		}

		s.logger.Info("init routine completed")
	}()

	go func() {
		// TODO make this a config option ?
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				ctx, cleanup := context.WithTimeout(parent, 10*time.Second)

				if err := s.sync(ctx); err != nil {
					s.logger.Errorf("failed to sync sourcemaps metadata: %v", err)
				}

				cleanup()
			case <-parent.Done():
				s.logger.Info("update routine done")
				return
			}
		}
	}()
}

func (s *MetadataCachingFetcher) sync(ctx context.Context) error {
	updates := make(map[Identifier]string)

	result, err := s.initialSearch(ctx, updates)
	if err != nil {
		return err
	}

	scrollID := result.ScrollID

	if scrollID == "" {
		return nil
	}

	for {
		result, err = s.scrollsearch(ctx, scrollID, updates)
		if err != nil {
			return err
		}

		// From the docs: The initial search request and each subsequent scroll
		// request each return a _scroll_id. While the _scroll_id may change between
		// requests, it doesn’t always change — in any case, only the most recently
		// received _scroll_id should be used.
		if result.ScrollID != "" {
			scrollID = result.ScrollID
		}

		// Stop if there are no new updates
		if len(result.Hits.Hits) == 0 {
			break
		}
	}

	s.update(ctx, updates)
	return nil
}

func (s *MetadataCachingFetcher) initialSearch(ctx context.Context, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runSearchQuery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataCachingFetcher) runSearchQuery(ctx context.Context) (*esapi.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(queryMetadata()); err != nil {
		return nil, err
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

func (s *MetadataCachingFetcher) handleUpdateRequest(resp *esapi.Response, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, errMsgParseSourcemap)
		}
		return nil, errors.New(fmt.Sprintf("%s %s", errMsgParseSourcemap, b))
	}

	// parse response
	body, err := parseResponse(resp.Body, s.logger)
	if err != nil {
		return nil, err
	}

	for _, v := range body.Hits.Hits {
		id := Identifier{
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
	hits := esSourcemapResponse.Hits.Total.Value
	if hits == 0 || len(esSourcemapResponse.Hits.Hits) == 0 {
		return &esSourcemapResponse, nil
	}

	return &esSourcemapResponse, nil
}

func (s *MetadataCachingFetcher) scrollsearch(ctx context.Context, scrollID string, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runScrollSearchQuery(ctx, scrollID)
	if err != nil {
		return nil, errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataCachingFetcher) runScrollSearchQuery(ctx context.Context, id string) (*esapi.Response, error) {
	req := esapi.ScrollRequest{
		ScrollID: id,
		Scroll:   time.Minute,
	}
	return req.Do(ctx, s.esClient)
}
