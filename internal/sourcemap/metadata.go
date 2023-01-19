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
	alias            map[Identifier]struct{}
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
		alias:            make(map[Identifier]struct{}),
		backend:          backend,
		logger:           logp.NewLogger(logs.Sourcemap),
		init:             make(chan struct{}),
		invalidationChan: invalidationChan,
	}
}

func (s *MetadataCachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var forwardRequest bool

	select {
	case <-s.init:
	default:
		s.logger.Debugf("Metadata cache not populated. Falling back to backend fetcher for id: %s, %s, %s", name, version, path)
		forwardRequest = true
	}

	keys := GetIdentifiers(name, version, path)

	if _, found := s.set[keys[0]]; found || forwardRequest {
		// Only fetch from ES if the sourcemap id exists
		c, err := s.backend.Fetch(ctx, keys[0].name, keys[0].version, keys[0].path)
		if c != nil || err != nil {
			return c, err
		}
	}

	for _, key := range keys[1:] {
		if _, found := s.alias[key]; found || forwardRequest {
			// Only fetch from ES if the sourcemap id exists
			c, err := s.backend.Fetch(ctx, key.name, key.version, key.path)
			if c != nil || err != nil {
				return c, err
			}
		}
	}

	return nil, nil
}

func (s *MetadataCachingFetcher) update(updates map[Identifier]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, contentHash := range s.set {
		if updatedHash, ok := updates[id]; ok {
			// already in the cache, remove from the updates.
			delete(updates, id)

			// content hash changed, invalidate the sourcemap cache
			if contentHash != updatedHash {
				s.invalidationChan <- id
			}
		} else {
			// the sourcemap no longer exists in ES.
			// invalidate the sourcemap cache.
			s.invalidationChan <- id

			// remove from metadata cache
			delete(s.set, id)
			// remove alias
			for _, k := range GetIdentifiers(id.name, id.version, id.path)[1:] {
				delete(s.alias, k)
			}
		}
	}
	// add new sourcemaps to the metadata cache.
	for id, contentHash := range updates {
		s.set[id] = contentHash
		for _, k := range GetIdentifiers(id.name, id.version, id.path)[1:] {
			s.alias[k] = struct{}{}
		}
	}
}

func (s *MetadataCachingFetcher) StartBackgroundSync() {
	go func() {
		// First run, populate cache
		ctx, cleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanup()

		if err := s.sync(ctx); err != nil {
			s.logger.Error("failed to fetch sourcemaps metadata: %v", err)
		}

		close(s.init)
	}()

	go func() {
		// TODO make this a config option ?
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

		for range t.C {
			ctx, cleanup := context.WithTimeout(context.Background(), 10*time.Second)

			if err := s.sync(ctx); err != nil {
				s.logger.Error("failed to sync sourcemaps metadata: %v", err)
			}

			cleanup()
		}
	}()
}

func (s *MetadataCachingFetcher) sync(ctx context.Context) error {
	resp, err := s.runSearchQuery(ctx)
	if err != nil {
		return errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, errMsgParseSourcemap)
		}
		return errors.New(fmt.Sprintf("%s %s", errMsgParseSourcemap, b))
	}

	// parse response
	body, err := parseResponse(resp.Body, s.logger)
	if err != nil {
		return err
	}

	updates := make(map[Identifier]string, len(body.Hits.Hits))
	for _, v := range body.Hits.Hits {
		id := Identifier{
			name:    v.Source.Service.Name,
			version: v.Source.Service.Version,
			path:    v.Source.File.BundleFilepath,
		}

		updates[id] = v.Source.ContentHash
	}

	// Update cache
	s.update(updates)
	return nil
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
	}
	return req.Do(ctx, s.esClient)
}

func queryMetadata() map[string]interface{} {
	return search(
		sources([]string{"service.*", "file.path", "content_sha256"}),
	)
}

func parseResponse(body io.ReadCloser, logger *logp.Logger) (esSourcemapResponse, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return esSourcemapResponse{}, err
	}

	var esSourcemapResponse esSourcemapResponse
	if err := json.Unmarshal(b, &esSourcemapResponse); err != nil {
		return esSourcemapResponse, err
	}
	hits := esSourcemapResponse.Hits.Total.Value
	if hits == 0 || len(esSourcemapResponse.Hits.Hits) == 0 {
		return esSourcemapResponse, nil
	}

	return esSourcemapResponse, nil
}
