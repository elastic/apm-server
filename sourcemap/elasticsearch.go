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
	"io/ioutil"
	"net/http"

	"github.com/go-sourcemap/sourcemap"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/go-elasticsearch/v7/esapi"

	"github.com/elastic/apm-server/elasticsearch"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/utility"
)

const (
	errMsgParseSourcemap = "Could not parse Sourcemap"
)

var (
	errMsgESFailure         = errMsgFailure + " ES"
	errSourcemapWrongFormat = errors.New("Sourcemapping ES Result not in expected format")
)

type esFetcher struct {
	client elasticsearch.Client
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
				Sourcemap struct {
					Sourcemap string
				}
			} `json:"_source"`
		}
	} `json:"hits"`
}

// NewElasticsearchFetcher returns a Fetcher for fetching source maps stored in Elasticsearch.
func NewElasticsearchFetcher(c elasticsearch.Client, index string) Fetcher {
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
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, errMsgParseSourcemap)
		}
		return nil, errors.New(fmt.Sprintf("%s %s", errMsgParseSourcemap, b))
	}

	// parse response
	body, err := parse(resp.Body, name, version, path, s.logger)
	if err != nil {
		return nil, err
	}
	return parseSourceMap(body)
}

func (s *esFetcher) runSearchQuery(ctx context.Context, name, version, path string) (*esapi.Response, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query(name, version, path)); err != nil {
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
	var buf bytes.Buffer
	if err := json.NewDecoder(io.TeeReader(body, &buf)).Decode(&esSourcemapResponse); err != nil {
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
	esSourcemap = esSourcemapResponse.Hits.Hits[0].Source.Sourcemap.Sourcemap
	// until https://github.com/golang/go/issues/19858 is resolved
	if esSourcemap == "" {
		return "", errSourcemapWrongFormat
	}
	return esSourcemap, nil
}

func query(name, version, path string) map[string]interface{} {
	return searchFirst(
		boolean(
			must(
				term("processor.name", "sourcemap"),
				term("sourcemap.service.name", name),
				term("sourcemap.service.version", version),
				term("processor.name", "sourcemap"),
				boolean(
					should(
						// prefer full URL match
						boostedTerm("sourcemap.bundle_filepath", path, 2.0),
						term("sourcemap.bundle_filepath", utility.UrlPath(path)),
					),
				),
			),
		),
		"sourcemap.sourcemap",
		desc("_score"),
		desc("@timestamp"),
	)
}

func wrap(k string, v map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{k: v}
}

func boolean(clause map[string]interface{}) map[string]interface{} {
	return wrap("bool", clause)
}

func should(clauses ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"should": clauses}
}

func must(clauses ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"must": clauses}
}

func term(k, v string) map[string]interface{} {
	return map[string]interface{}{"term": map[string]interface{}{k: v}}
}

func boostedTerm(k, v string, boost float32) map[string]interface{} {
	return wrap("term",
		wrap(k, map[string]interface{}{
			"value": v,
			"boost": boost,
		}),
	)
}

func desc(by string) map[string]interface{} {
	return wrap(by, map[string]interface{}{"order": "desc"})
}

func searchFirst(query map[string]interface{}, source string, sort ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"query":   query,
		"size":    1,
		"sort":    sort,
		"_source": source,
	}
}
