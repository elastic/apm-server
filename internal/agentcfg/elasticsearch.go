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
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.elastic.co/apm/v2"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const ElasticsearchIndexName = ".apm-agent-configuration"

const (
	// ErrInfrastructureNotReady is returned when a fetch request comes in while
	// the infrastructure is not ready to serve the request.
	// This may happen when the local cache is not initialized and no fallback fetcher is configured.
	ErrInfrastructureNotReady = "agentcfg infrastructure is not ready"

	// ErrNoValidElasticsearchConfig is an error where the server is
	// not properly configured to fetch agent configuration.
	ErrNoValidElasticsearchConfig = "no valid elasticsearch config to fetch agent config"
)

const (
	refreshCacheTimeout = 5 * time.Second
	loggerRateLimit     = time.Minute
)

type ElasticsearchFetcher struct {
	client          *elasticsearch.Client
	cacheDuration   time.Duration
	fallbackFetcher Fetcher

	mu    sync.RWMutex
	last  time.Time
	cache []AgentConfig

	searchSize int

	invalidESCfg     atomic.Bool
	cacheInitialized atomic.Bool

	logger, rateLimitedLogger *logp.Logger

	tracer  *apm.Tracer
	metrics fetcherMetrics
}

type fetcherMetrics struct {
	fetchES, fetchFallback, fetchFallbackUnavailable, fetchInvalid,
	cacheRefreshSuccesses, cacheRefreshFailures,
	cacheEntriesCount atomic.Int64
}

func NewElasticsearchFetcher(
	client *elasticsearch.Client,
	cacheDuration time.Duration,
	fetcher Fetcher,
	tracer *apm.Tracer,
) *ElasticsearchFetcher {
	logger := logp.NewLogger("agentcfg")
	return &ElasticsearchFetcher{
		client:            client,
		cacheDuration:     cacheDuration,
		fallbackFetcher:   fetcher,
		searchSize:        100,
		logger:            logger,
		rateLimitedLogger: logger.WithOptions(logs.WithRateLimit(loggerRateLimit)),
		tracer:            tracer,
	}
}

// Fetch finds a matching agent config based on the received query.
func (f *ElasticsearchFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	if f.cacheInitialized.Load() {
		// Happy path: serve fetch requests using an initialized cache.
		f.mu.RLock()
		defer f.mu.RUnlock()
		f.metrics.fetchES.Add(1)
		return matchAgentConfig(query, f.cache), nil
	}

	if f.fallbackFetcher != nil {
		f.metrics.fetchFallback.Add(1)
		return f.fallbackFetcher.Fetch(ctx, query)
	}

	if f.invalidESCfg.Load() {
		f.metrics.fetchInvalid.Add(1)
		f.rateLimitedLogger.Errorf("rejecting fetch request: no valid elasticsearch config")
		return Result{}, errors.New(ErrNoValidElasticsearchConfig)
	}

	f.metrics.fetchFallbackUnavailable.Add(1)
	f.rateLimitedLogger.Warnf("rejecting fetch request: infrastructure is not ready")
	return Result{}, errors.New(ErrInfrastructureNotReady)
}

// Run refreshes the fetcher cache by querying Elasticsearch periodically.
func (f *ElasticsearchFetcher) Run(ctx context.Context) error {
	refresh := func() bool {
		// refresh returns a bool that indicates whether Run should return
		// immediately without error, e.g. due to invalid Elasticsearch config.
		tx := f.tracer.StartTransaction("ElasticsearchFetcher.refresh", "")
		defer tx.End()
		ctx = apm.ContextWithTransaction(ctx, tx)

		if err := f.refreshCache(ctx); err != nil {
			if e := apm.CaptureError(ctx, err); e != nil {
				e.Send()
			}

			// Do not log as error when there is a fallback.
			var logFunc func(string, ...interface{})
			if f.fallbackFetcher == nil {
				logFunc = f.logger.Errorf
			} else {
				logFunc = f.logger.Warnf
			}

			logFunc("refresh cache error: %s", err)
			if f.invalidESCfg.Load() {
				logFunc("stopping refresh cache background job: elasticsearch config is invalid")
				return true
			}
		} else {
			f.logger.Debugf("refresh cache success")
		}
		return false
	}

	// Trigger initial run.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if stop := refresh(); stop {
			return nil
		}
	}

	// Then schedule subsequent runs.
	t := time.NewTicker(f.cacheDuration)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if stop := refresh(); stop {
				return nil
			}
		}
	}
}

type cacheResult struct {
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

func (f *ElasticsearchFetcher) refreshCache(ctx context.Context) (err error) {
	span, ctx := apm.StartSpan(ctx, "ElasticsearchFetcher.refreshCache", "")
	defer span.End()

	scrollID := ""
	buffer := make([]AgentConfig, 0, len(f.cache))

	// The refresh cache operation should complete within refreshCacheTimeout.
	ctx, cancel := context.WithTimeout(ctx, refreshCacheTimeout)
	defer cancel()

	defer func() {
		if err != nil {
			f.metrics.cacheRefreshFailures.Add(1)
		} else {
			f.metrics.cacheRefreshSuccesses.Add(1)
		}
	}()

	for {
		result, err := f.singlePageRefresh(ctx, scrollID)
		if err != nil {
			return err
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
	f.cache = buffer
	f.mu.Unlock()
	f.cacheInitialized.Store(true)
	f.metrics.cacheEntriesCount.Store(int64(len(f.cache)))
	f.last = time.Now()
	return nil
}

func (f *ElasticsearchFetcher) singlePageRefresh(ctx context.Context, scrollID string) (cacheResult, error) {
	var result cacheResult

	if scrollID == "" {
		resp, err := esapi.SearchRequest{
			Index:  []string{ElasticsearchIndexName},
			Size:   &f.searchSize,
			Scroll: f.cacheDuration,
		}.Do(ctx, f.client)
		if err != nil {
			return result, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			// Elasticsearch returns 401 on unauthorized requests and 403 on insufficient permission
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				f.invalidESCfg.Store(true)
			}
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				f.logger.Debugf("refresh cache elasticsearch returned status %d: %s", resp.StatusCode, string(bodyBytes))
			}
			return result, fmt.Errorf("refresh cache elasticsearch returned status %d", resp.StatusCode)
		}
		return result, json.NewDecoder(resp.Body).Decode(&result)
	}

	resp, err := esapi.ScrollRequest{
		ScrollID: result.ScrollID,
		Scroll:   f.cacheDuration,
	}.Do(ctx, f.client)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	return result, json.NewDecoder(resp.Body).Decode(&result)
}

// CollectMonitoring may be called to collect monitoring metrics from the
// fetcher. It is intended to be used with libbeat/monitoring.NewFunc.
//
// The metrics should be added to the "apm-server.agentcfg.elasticsearch" registry.
func (f *ElasticsearchFetcher) CollectMonitoring(_ monitoring.Mode, V monitoring.Visitor) {
	V.OnRegistryStart()
	defer V.OnRegistryFinished()

	monitoring.ReportInt(V, "cache.entries.count", f.metrics.cacheEntriesCount.Load())
	monitoring.ReportInt(V, "fetch.es", f.metrics.fetchES.Load())
	monitoring.ReportInt(V, "fetch.fallback", f.metrics.fetchFallback.Load())
	monitoring.ReportInt(V, "fetch.unavailable", f.metrics.fetchFallbackUnavailable.Load())
	monitoring.ReportInt(V, "fetch.invalid", f.metrics.fetchInvalid.Load())
	monitoring.ReportInt(V, "cache.refresh.successes", f.metrics.cacheRefreshSuccesses.Load())
	monitoring.ReportInt(V, "cache.refresh.failures", f.metrics.cacheRefreshFailures.Load())
}
