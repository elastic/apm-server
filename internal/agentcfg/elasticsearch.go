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
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const ElasticsearchIndexName = ".apm-agent-configuration"

const (
	// ErrInfrastructureNotReady is returned when a fetch request comes in while
	// the first run of refresh cache has not completed yet.
	ErrInfrastructureNotReady = "agentcfg infrastructure is not ready"

	// ErrNoValidElasticsearchConfig is an error where the server is
	// not properly configured to fetch agent configuration.
	ErrNoValidElasticsearchConfig = "no valid elasticsearch config to fetch agent config"
)

const refreshCacheTimeout = 5 * time.Second

type ElasticsearchFetcher struct {
	client          *elasticsearch.Client
	cacheDuration   time.Duration
	fallbackFetcher Fetcher

	mu    sync.RWMutex
	last  time.Time
	cache []AgentConfig

	searchSize int

	firstRunCompleted rwFlag
	invalidESCfg      rwFlag
	cacheInitialized  rwFlag

	logger *logp.Logger

	metrics fetcherMetrics
}

type rwFlag struct {
	mu  sync.RWMutex
	val bool
}

func (r *rwFlag) Get() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.val
}

func (r *rwFlag) Set() {
	if !r.Get() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.val = true
	}
}

type fetcherMetrics struct {
	fetchES, fetchFallback, fetchFallbackUnavailable int64
	cacheRefreshSuccesses, cacheRefreshFailures      int64
	cacheEntriesCount                                int64
}

func NewElasticsearchFetcher(client *elasticsearch.Client, cacheDuration time.Duration, fetcher Fetcher) *ElasticsearchFetcher {
	logger := logp.NewLogger("agentcfg")
	return &ElasticsearchFetcher{
		client:          client,
		cacheDuration:   cacheDuration,
		fallbackFetcher: fetcher,
		searchSize:      100,
		logger:          logger,
	}
}

// Fetch finds a matching agent config based on the received query.
func (f *ElasticsearchFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	if f.cacheInitialized.Get() {
		// Happy path: serve fetch requests using an initialized cache.
		f.mu.RLock()
		defer f.mu.RUnlock()
		defer atomic.AddInt64(&f.metrics.fetchES, 1)
		return matchAgentConfig(query, f.cache), nil
	}

	if !f.firstRunCompleted.Get() {
		f.logger.Warnf("rejecting fetch request: first refresh cache run has not completed")
		return Result{}, errors.New(ErrInfrastructureNotReady)
	} else {
		if f.fallbackFetcher != nil {
			defer atomic.AddInt64(&f.metrics.fetchFallback, 1)
			return f.fallbackFetcher.Fetch(ctx, query)
		} else if f.fallbackFetcher == nil && f.invalidESCfg.Get() {
			defer atomic.AddInt64(&f.metrics.fetchFallbackUnavailable, 1)
			f.logger.Errorf("rejecting fetch request: no valid elasticsearch config")
			return Result{}, errors.New(ErrNoValidElasticsearchConfig)
		} else {
			f.logger.Warnf("rejecting fetch request: infrastructure is not ready")
			return Result{}, errors.New(ErrInfrastructureNotReady)
		}
	}
}

// Run refreshes the fetcher cache by querying Elasticsearch periodically.
func (f *ElasticsearchFetcher) Run(ctx context.Context) error {
	t := time.NewTicker(f.cacheDuration)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}

		if err := f.refreshCache(ctx); err != nil {
			// Do not log as error when there is a fallback.
			var logFunc func(string, ...interface{})
			if f.fallbackFetcher == nil {
				logFunc = f.logger.Errorf
			} else {
				logFunc = f.logger.Warnf
			}

			logFunc("refresh cache error: %s", err)
			if f.invalidESCfg.Get() {
				logFunc("stopping refresh cache background job: elasticsearch config is invalid")
				return nil
			}
		} else {
			f.logger.Debugf("refresh cache success")
		}
	}
}

func (f *ElasticsearchFetcher) refreshCache(ctx context.Context) (err error) {
	scrollID := ""
	buffer := make([]AgentConfig, 0, len(f.cache))

	// The refresh cache operation should complete within refreshCacheTimeout.
	ctx, cancel := context.WithTimeout(ctx, refreshCacheTimeout)
	defer cancel()

	// Flag that first run has completed regardless of outcome.
	defer f.firstRunCompleted.Set()

	defer func() {
		if err != nil {
			atomic.AddInt64(&f.metrics.cacheRefreshFailures, 1)
		} else {
			atomic.AddInt64(&f.metrics.cacheRefreshSuccesses, 1)
		}
	}()

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
		ScrollID string `json:"_scroll_id"`
	}

	for {
		err = func() error {
			var (
				resp *esapi.Response
				err  error
			)
			if scrollID == "" {
				resp, err = esapi.SearchRequest{
					Index:  []string{ElasticsearchIndexName},
					Size:   &f.searchSize,
					Scroll: f.cacheDuration,
				}.Do(ctx, f.client)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode >= http.StatusBadRequest {
					// Elasticsearch returns 401 on unauthorized requests and 403 on insufficient permission
					if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
						f.invalidESCfg.Set()
					}
					return fmt.Errorf("refresh cache returns non-200 status: %d", resp.StatusCode)
				}
			} else {
				resp, err = esapi.ScrollRequest{
					ScrollID: result.ScrollID,
					Scroll:   f.cacheDuration,
				}.Do(ctx, f.client)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return
		}

		for _, hit := range result.Hits.Hits {
			buffer = append(buffer, AgentConfig{
				ServiceName:        hit.Source.Service.Name,
				ServiceEnvironment: hit.Source.Service.Environment,
				AgentName:          hit.Source.AgentName,
				Etag:               hit.Source.ETag,
				Config:             hit.Source.Settings,
			})
		}
		scrollID = result.ScrollID
		if len(result.Hits.Hits) == 0 {
			break
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = buffer
	f.cacheInitialized.Set()
	atomic.StoreInt64(&f.metrics.cacheEntriesCount, int64(len(f.cache)))
	f.last = time.Now()
	return nil
}

// CollectMonitoring may be called to collect monitoring metrics from the
// fetcher. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.agentcfg.elasticsearch" registry.
func (f *ElasticsearchFetcher) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	monitoring.ReportInt(V, "cache.entries.count", atomic.LoadInt64(&f.metrics.cacheEntriesCount))
	monitoring.ReportInt(V, "fetch.es", atomic.LoadInt64(&f.metrics.fetchES))
	monitoring.ReportInt(V, "fetch.fallback", atomic.LoadInt64(&f.metrics.fetchFallback))
	monitoring.ReportInt(V, "fetch.unavailable", atomic.LoadInt64(&f.metrics.fetchFallbackUnavailable))
	monitoring.ReportInt(V, "cache.refresh.successes", atomic.LoadInt64(&f.metrics.cacheRefreshSuccesses))
	monitoring.ReportInt(V, "cache.refresh.failures", atomic.LoadInt64(&f.metrics.cacheRefreshFailures))
}
