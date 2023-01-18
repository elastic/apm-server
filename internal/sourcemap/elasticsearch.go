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
	"fmt"
	"io"
	"net/http"

	"github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
)

const (
	errMsgParseSourcemap = "Could not parse Sourcemap"
)

var (
	errMsgESFailure         = errMsgFailure + " ES"
	errSourcemapWrongFormat = errors.New("Sourcemapping ES Result not in expected format")
)

type esFetcher struct {
	client *elasticsearch.Client
	index  string
	logger *logp.Logger
}

type esSourcemapResponse struct {
	Hits struct {
		Total struct {
			Value int
		}
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

// NewElasticsearchFetcher returns a Fetcher for fetching source maps stored in Elasticsearch.
func NewElasticsearchFetcher(c *elasticsearch.Client, index string) Fetcher {
	logger := logp.NewLogger(logs.Sourcemap)
	return &esFetcher{c, index, logger}
}

// Fetch fetches a source map from Elasticsearch.
func (s *esFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	resp, err := s.runSearchQuery(ctx, name, version, path)
	if err != nil {
		return nil, errors.Wrap(err, errMsgESFailure)
	}
	defer resp.Body.Close()

	// handle error response
	if resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, errMsgParseSourcemap)
		}
		return nil, errors.New(fmt.Sprintf("%s %s", errMsgParseSourcemap, body))
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

	return parseSourceMap(string(uncompressedBody))
}

func (s *esFetcher) runSearchQuery(ctx context.Context, name, version, path string) (*esapi.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(requestBody(name, version, path)); err != nil {
		return nil, err
	}
	req := esapi.SearchRequest{
		Index:          []string{s.index},
		Body:           &buf,
		TrackTotalHits: true,
	}
	return req.Do(ctx, s.client)
}

func parse(body io.ReadCloser, name, version, path string, logger *logp.Logger) (string, error) {
	var esSourcemapResponse esSourcemapResponse
	if err := json.NewDecoder(body).Decode(&esSourcemapResponse); err != nil {
		return "", err
	}
	hits := esSourcemapResponse.Hits.Total.Value
	if hits == 0 || len(esSourcemapResponse.Hits.Hits) == 0 {
		return "", nil
	}

	var esSourcemap string
	if hits > 1 {
		logger.Warnf("%d sourcemaps found for service %s version %s and file %s, using the most recent one",
			hits, name, version, path)
	}
	esSourcemap = esSourcemapResponse.Hits.Hits[0].Source.Sourcemap
	// until https://github.com/golang/go/issues/19858 is resolved
	if esSourcemap == "" {
		return "", errSourcemapWrongFormat
	}
	return esSourcemap, nil
}

func requestBody(name, version, path string) map[string]interface{} {
	id := name + "-" + version + "-" + path
	return search(
		size(1),
		sort(desc("_score")),
		source("content"),
		query(
			boolean(
				must(
					term("_id", id),
				),
			),
		),
	)
}
