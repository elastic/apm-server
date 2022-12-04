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

type Identifier struct {
	name    string
	version string
	path    string
}

type Metadata struct {
	id          Identifier
	contentHash string
}

type MetadataCachingFetcher struct {
	esClient elasticsearch.Client
	set      map[Identifier]bool
	mu       sync.RWMutex
	backend  Fetcher
	logger   *logp.Logger
	once     sync.Once
	index    string
}

func NewMetadataCachingFetcher(
	c elasticsearch.Client,
	backend Fetcher,
	index string,
) *MetadataCachingFetcher {
	return &MetadataCachingFetcher{
		esClient: c,
		index:    index,
		set:      make(map[Identifier]bool),
		backend:  backend,
		logger:   logp.NewLogger(logs.Sourcemap),
	}
}

func (s *MetadataCachingFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	if len(s.set) == 0 {
		// First run, populate cache
		s.once.Do(func() {
			s.sync(ctx)
		})
	}

	key := Identifier{
		name:    name,
		version: version,
		path:    path,
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, found := s.set[key]; found {
		// Only fetch from ES if the sourcemap id exists
		return s.backend.Fetch(ctx, name, version, path)
	}

	return nil, nil
}

func (s *MetadataCachingFetcher) Update(ids []Identifier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k := range s.set {
		delete(s.set, k)
	}

	for _, k := range ids {
		s.set[k] = true
	}
}

func (s *MetadataCachingFetcher) StartBackgroundSync() {
	go func() {
		// TODO make this a config option ?
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()

		for range t.C {
			ctx, cleanup := context.WithTimeout(context.Background(), 10*time.Second)
			s.sync(ctx)
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

	var ids []Identifier
	for _, v := range body.Hits.Hits {
		id := Identifier{
			name:    v.Source.Sourcemap.Service.Name,
			version: v.Source.Sourcemap.Service.Version,
			path:    v.Source.Sourcemap.BundleFilepath,
		}

		ids = append(ids, id)
	}

	// Update cache
	s.Update(ids)
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
	return map[string]interface{}{
		"_source": []string{"sourcemap.service.*", "sourcemap.bundle_filepath"},
	}
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
