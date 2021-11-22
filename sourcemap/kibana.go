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
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-sourcemap/sourcemap"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
)

const sourcemapArtifactType = "sourcemap"

type kibanaFetcher struct {
	client kibana.Client
	logger *logp.Logger
}

type kibanaSourceMapArtifact struct {
	Type string `json:"type"`
	Body struct {
		ServiceName    string          `json:"serviceName"`
		ServiceVersion string          `json:"serviceVersion"`
		BundleFilepath string          `json:"bundleFilepath"`
		SourceMap      json.RawMessage `json:"sourceMap"`
	} `json:"body"`
}

// NewKibanaFetcher returns a Fetcher that fetches source maps stored by Kibana.
func NewKibanaFetcher(c kibana.Client) Fetcher {
	logger := logp.NewLogger(logs.Sourcemap)
	return &kibanaFetcher{c, logger}
}

// Fetch fetches a source map from Kibana.
func (s *kibanaFetcher) Fetch(ctx context.Context, name, version, path string) (*sourcemap.Consumer, error) {
	resp, err := s.client.Send(ctx, "GET", "/api/apm/sourcemaps", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to query source maps (%s): %s", resp.Status, body)
	}

	var result struct {
		Artifacts []kibanaSourceMapArtifact `json:"artifacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	path = maybeParseURLPath(path)
	for _, a := range result.Artifacts {
		if a.Type != sourcemapArtifactType {
			continue
		}
		if a.Body.ServiceName == name && a.Body.ServiceVersion == version && maybeParseURLPath(a.Body.BundleFilepath) == path {
			return parseSourceMap(string(a.Body.SourceMap))
		}
	}
	return nil, nil
}
