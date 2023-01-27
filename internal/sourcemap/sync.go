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
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	syncTimeout = 10 * time.Second
)

type SyncWorker struct {
	esClient   *elasticsearch.Client
	logger     *logp.Logger
	index      string
	updateChan chan<- map[Identifier]string
}

func NewSyncWorker(esClient *elasticsearch.Client, index string) (*SyncWorker, <-chan map[Identifier]string) {
	ch := make(chan map[Identifier]string)
	return &SyncWorker{
		esClient:   esClient,
		index:      index,
		logger:     logp.NewLogger(logs.Sourcemap),
		updateChan: ch,
	}, ch
}

func (s *SyncWorker) Run(parent context.Context) {
	go func() {
		// First run, populate cache
		ctx, cleanup := context.WithTimeout(parent, syncTimeout)
		defer cleanup()

		if err := s.sync(ctx); err != nil {
			s.logger.Errorf("failed to fetch sourcemaps metadata: %v", err)
			// send nil to the update channel so that listeners don't block forever
			s.updateChan <- nil
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
				ctx, cleanup := context.WithTimeout(parent, syncTimeout)

				if err := s.sync(ctx); err != nil {
					s.logger.Errorf("failed to sync sourcemaps metadata: %v", err)
				}

				cleanup()
			case <-parent.Done():
				s.logger.Info("update routine done")
				// close update channel
				close(s.updateChan)
				return
			}
		}
	}()
}

func (s *SyncWorker) sync(ctx context.Context) error {
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

	select {
	case s.updateChan <- updates:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *SyncWorker) initialSearch(ctx context.Context, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runSearchQuery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *SyncWorker) runSearchQuery(ctx context.Context) (*esapi.Response, error) {
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

func (s *SyncWorker) handleUpdateRequest(resp *esapi.Response, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
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

func (s *SyncWorker) scrollsearch(ctx context.Context, scrollID string, updates map[Identifier]string) (*esSearchSourcemapResponse, error) {
	resp, err := s.runScrollSearchQuery(ctx, scrollID)
	if err != nil {
		return nil, errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	return s.handleUpdateRequest(resp, updates)
}

func (s *SyncWorker) runScrollSearchQuery(ctx context.Context, id string) (*esapi.Response, error) {
	req := esapi.ScrollRequest{
		ScrollID: id,
		Scroll:   time.Minute,
	}
	return req.Do(ctx, s.esClient)
}
