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
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
)

type esFetcher struct {
	client *elasticsearch.Client
	index  string
	logger *logp.Logger
}

type esGetSourcemapResponse struct {
	Found  bool `json:"found"`
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
}

// NewElasticsearchFetcher returns a Fetcher for fetching source maps stored in Elasticsearch.
func NewElasticsearchFetcher(c *elasticsearch.Client, index string) Fetcher {
	logger := logp.NewLogger(logs.Sourcemap)
	return &esFetcher{c, index, logger}
}

// Fetch fetches a source map from Elasticsearch.
func (s *esFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	resp, err := s.runSearchQuery(ctx, name, version, path)
	if err != nil {
		var networkErr net.Error
		if errors.As(err, &networkErr) {
			return nil, fmt.Errorf("failed to reach elasticsearch: %w: %v ", errFetcherUnvailable, err)
		}
		return nil, fmt.Errorf("failure querying ES: %w", err)
	}
	defer resp.Body.Close()

	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read ES response body: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			// http.StatusNotFound -> the index is missing
			// http.StatusForbidden -> we don't have permission to read from the index
			// In both cases we consider the fetcher unavailable so that APM Server can
			// fallback to other fetchers
			return nil, fmt.Errorf("%w: %s: %s", errFetcherUnvailable, resp.Status(), string(b))
		}
		return nil, fmt.Errorf("ES returned unknown status code: %s", resp.Status())
	}

	// parse response
	body, err := parse(resp.Body, name, version, path, s.logger)
	if err != nil {
		return nil, err
	}

	if body == "" {
		return nil, nil
	}

	decodedBody, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode string: %w", err)
	}

	r, err := zlib.NewReader(bytes.NewReader(decodedBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer r.Close()

	uncompressedBody, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read sourcemap content: %w", err)
	}

	return parseSourceMap(uncompressedBody)
}

func (s *esFetcher) runSearchQuery(ctx context.Context, name, version, path string) (*esapi.Response, error) {
	id := name + "-" + version + "-" + path
	req := esapi.GetRequest{
		Index:      s.index,
		DocumentID: url.PathEscape(id),
	}
	return req.Do(ctx, s.client)
}

func parse(body io.ReadCloser, name, version, path string, logger *logp.Logger) (string, error) {
	var esSourcemapResponse esGetSourcemapResponse
	if err := json.NewDecoder(body).Decode(&esSourcemapResponse); err != nil {
		return "", fmt.Errorf("failed to decode sourcemap: %w", err)
	}

	if !esSourcemapResponse.Found {
		return "", nil
	}

	return esSourcemapResponse.Source.Sourcemap, nil
}
