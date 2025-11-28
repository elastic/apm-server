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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

type MetadataESFetcher struct {
	esClient         *elasticsearch.Client
	index            string
	set              map[identifier]string
	alias            map[identifier]*identifier
	mu               sync.RWMutex
	logger           *logp.Logger
	init             chan struct{}
	initErr          error
	invalidationChan chan<- []identifier
	tracer           trace.Tracer
}

func NewMetadataFetcher(
	ctx context.Context,
	esClient *elasticsearch.Client,
	index string,
	tp trace.TracerProvider,
	logger *logp.Logger,
) (MetadataFetcher, <-chan []identifier) {
	invalidationCh := make(chan []identifier)

	tracer := tp.Tracer("github.com/elastic/apm-server/internal/sourcemap")
	s := &MetadataESFetcher{
		esClient:         esClient,
		index:            index,
		set:              make(map[identifier]string),
		alias:            make(map[identifier]*identifier),
		logger:           logger.Named(logs.Sourcemap),
		init:             make(chan struct{}),
		invalidationChan: invalidationCh,
		tracer:           tracer,
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

func (s *MetadataESFetcher) err() error {
	select {
	case <-s.ready():
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.initErr
	default:
		return errors.New("metadata es fetcher not ready")
	}
}

func (s *MetadataESFetcher) startBackgroundSync(ctx context.Context) {
	go func() {
		s.logger.Debug("populating metadata cache")

		// First run, populate cache
		if err := s.sync(ctx); err != nil {
			s.initErr = fmt.Errorf("failed to populate sourcemap metadata: %w", err)
			s.logger.Error(s.initErr)
		}

		close(s.init)

		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				if err := s.sync(ctx); err != nil {
					s.logger.Errorf("failed to sync sourcemaps metadata: %v", err)
				}
			case <-ctx.Done():
				s.logger.Info("update routine done")
				// close invalidation channel
				close(s.invalidationChan)
				return
			}
		}
	}()
}

func (s *MetadataESFetcher) sync(ctx context.Context) error {
	ctx, tx := s.tracer.Start(ctx, "MetadataESFetcher.sync", trace.WithSpanKind(trace.SpanKindInternal))
	defer tx.End()

	sourcemaps := make(map[identifier]string)

	result, err := s.initialSearch(ctx, sourcemaps)
	if err != nil {
		tx.RecordError(err)
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
			s.clearScroll(ctx, scrollID)
			tx.RecordError(err)
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

	s.clearScroll(ctx, scrollID)

	s.update(ctx, sourcemaps)
	return nil
}

func (s *MetadataESFetcher) clearScroll(ctx context.Context, scrollID string) {
	if scrollID == "" {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "/_search/scroll/"+scrollID, nil)
	if err != nil {
		s.logger.Warnf("failed to clear scroll: %v", err)
		return
	}

	resp, err := s.esClient.Perform(req)
	if err != nil {
		s.logger.Warnf("failed to clear scroll: %v", err)
		return
	}

	if resp.StatusCode > 299 {
		s.logger.Warnf("clearscroll request returned error: %s", resp.Status)
	}

	resp.Body.Close()
}

func (s *MetadataESFetcher) update(ctx context.Context, sourcemaps map[identifier]string) {
	ctx, span := s.tracer.Start(ctx, "MetadataESFetcher.update", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	var invalidation []identifier

	for id, contentHash := range s.set {
		if updatedHash, ok := sourcemaps[id]; ok {
			if contentHash == updatedHash {
				// already in the cache, remove from the updates.
				delete(sourcemaps, id)
			} else {
				// content hash changed, invalidate the sourcemap cache
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

	if len(invalidation) != 0 {
		select {
		case s.invalidationChan <- invalidation:
		case <-ctx.Done():
			s.logger.Debug("timed out while invalidating soucemaps")
		}
	}

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

	s.initErr = nil

	s.logger.Debugf("Metadata cache now has %d entries.", len(s.set))
}

func (s *MetadataESFetcher) initialSearch(ctx context.Context, updates map[identifier]string) (*esSearchSourcemapResponse, error) {
	ctx, span := s.tracer.Start(ctx, "MetadataESFetcher.initialSearch", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	resp, err := s.runSearchQuery(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to run initial search query: %w: %v", errFetcherUnvailable, err)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataESFetcher) runSearchQuery(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/"+s.index+"/_search", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("_source", strings.Join([]string{"service.*", "file.path", "content_sha256"}, ","))
	q.Set("track_total_hits", "true")
	q.Set("scroll", strconv.FormatInt(time.Minute.Milliseconds(), 10)+"ms")
	req.URL.RawQuery = q.Encode()

	return s.esClient.Perform(req)
}

type esSearchSourcemapResponse struct {
	ScrollID string `json:"_scroll_id"`
	esSourcemapResponse
}

type esSourcemapResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source struct {
				Service struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"service"`
				File struct {
					BundleFilepath string `json:"path"`
				} `json:"file"`
				Sourcemap   string `json:"content"`
				ContentHash string `json:"content_sha256"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func (s *MetadataESFetcher) handleUpdateRequest(resp *http.Response, updates map[identifier]string) (*esSearchSourcemapResponse, error) {
	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %s: %s", errFetcherUnvailable, resp.Status, string(b))
		}
		return nil, fmt.Errorf("ES returned unknown status code: %s", resp.Status)
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
	ctx, span := s.tracer.Start(ctx, "MetadataESFetcher.scrollSearch", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	resp, err := s.runScrollSearchQuery(ctx, scrollID)
	if err != nil {
		return nil, fmt.Errorf("failed to run scroll search query: %w", err)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *MetadataESFetcher) runScrollSearchQuery(ctx context.Context, id string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/_search/scroll", nil)
	q := req.URL.Query()
	q.Set("scroll", strconv.FormatInt(time.Minute.Milliseconds(), 10)+"ms")
	q.Set("scroll_id", id)
	req.URL.RawQuery = q.Encode()
	if err != nil {
		return nil, err
	}
	return s.esClient.Perform(req)
}
