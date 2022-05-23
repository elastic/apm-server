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

package agentcfg

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type ElasticsearchFetcher struct {
	client        elasticsearch.Client
	cacheDuration time.Duration

	mu    sync.RWMutex
	last  time.Time
	cache []config.AgentConfig
}

func NewElasticsearchFetcher(client elasticsearch.Client, cacheDuration time.Duration) *ElasticsearchFetcher {
	return &ElasticsearchFetcher{
		client:        client,
		cacheDuration: cacheDuration,
	}
}

// Fetch finds a matching agent config based on the received query,
// fetching agent configuration directly from Elasticsearch.
func (f *ElasticsearchFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	f.mu.RLock()
	if time.Since(f.last) < f.cacheDuration {
		defer f.mu.RUnlock()
		return matchAgentConfig(query, f.cache), nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Since(f.last) < f.cacheDuration {
		return matchAgentConfig(query, f.cache), nil
	}

	return matchAgentConfig(query, f.cache), nil
}

func (f *ElasticsearchFetcher) refreshCache(ctx context.Context) error {
	// TODO(axw) scroll search
	size := 1000
	resp, err := esapi.SearchRequest{
		Index:          []string{".apm-agent-configuration"},
		Size:           &size,
		TrackTotalHits: false,
	}.Do(ctx, f.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					AgentName string `json:"agent_name"`
					ETag      string `json:"etag"`
					Service   struct {
						Name        string `json:"name"`
						Environment string `json:"environment"`
					} `json:"service"`
					Settings map[string]string `json:"settings"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	f.cache = f.cache[:0]
	for _, hit := range result.Hits.Hits {
		f.cache = append(f.cache, config.AgentConfig{
			Service: config.Service{
				Name:        hit.Source.Service.Name,
				Environment: hit.Source.Service.Environment,
			},
			AgentName: hit.Source.AgentName,
			Etag:      hit.Source.ETag,
			Config:    hit.Source.Settings,
		})
	}
	f.last = time.Now()
	return nil
}
