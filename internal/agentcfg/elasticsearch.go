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
	"go.opentelemetry.io/otel/metric"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
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

	tracer *apm.Tracer

	esCacheEntriesCount     metric.Int64Gauge
	esFetchCount            metric.Int64Counter
	esFetchFallbackCount    metric.Int64Counter
	esFetchUnavailableCount metric.Int64Counter
	esFetchInvalidCount     metric.Int64Counter
	esCacheRefreshSuccesses metric.Int64Counter
	esCacheRefreshFailures  metric.Int64Counter
}

func NewElasticsearchFetcher(
	client *elasticsearch.Client,
	cacheDuration time.Duration,
	fetcher Fetcher,
	tracer *apm.Tracer,
	mp metric.MeterProvider,
) *ElasticsearchFetcher {
	meter := mp.Meter("github.com/elastic/apm-server/internal/agentcfg")

	esCacheEntriesCount, _ := meter.Int64Gauge("apm-server.agentcfg.elasticsearch.cache.entries.count")
	esFetchCount, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.fetch.es")
	esFetchFallbackCount, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.fetch.fallback")
	esFetchUnavailableCount, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.fetch.unavailable")
	esFetchInvalidCount, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.fetch.invalid")
	esCacheRefreshSuccesses, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.cache.refresh.successes")
	esCacheRefreshFailures, _ := meter.Int64Counter("apm-server.agentcfg.elasticsearch.cache.refresh.failures")

	logger := logp.NewLogger("agentcfg")
	return &ElasticsearchFetcher{
		client:            client,
		cacheDuration:     cacheDuration,
		fallbackFetcher:   fetcher,
		searchSize:        100,
		logger:            logger,
		rateLimitedLogger: logger.WithOptions(logs.WithRateLimit(loggerRateLimit)),
		tracer:            tracer,

		esCacheEntriesCount:     esCacheEntriesCount,
		esFetchCount:            esFetchCount,
		esFetchFallbackCount:    esFetchFallbackCount,
		esFetchUnavailableCount: esFetchUnavailableCount,
		esFetchInvalidCount:     esFetchInvalidCount,
		esCacheRefreshSuccesses: esCacheRefreshSuccesses,
		esCacheRefreshFailures:  esCacheRefreshFailures,
	}
}

// Fetch finds a matching agent config based on the received query.
func (f *ElasticsearchFetcher) Fetch(ctx context.Context, query Query) (Result, error) {
	if f.cacheInitialized.Load() {
		// Happy path: serve fetch requests using an initialized cache.
		f.mu.RLock()
		defer f.mu.RUnlock()
		f.esFetchCount.Add(ctx, 1)
		return matchAgentConfig(query, f.cache), nil
	}

	if f.fallbackFetcher != nil {
		f.esFetchFallbackCount.Add(ctx, 1)
		return f.fallbackFetcher.Fetch(ctx, query)
	}

	if f.invalidESCfg.Load() {
		f.esFetchInvalidCount.Add(ctx, 1)
		f.rateLimitedLogger.Errorf("rejecting fetch request: no valid elasticsearch config")
		return Result{}, errors.New(ErrNoValidElasticsearchConfig)
	}

	f.esFetchUnavailableCount.Add(ctx, 1)
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
			f.esCacheRefreshFailures.Add(ctx, 1)
		} else {
			f.esCacheRefreshSuccesses.Add(ctx, 1)
		}
	}()

	for {
		result, err := f.singlePageRefresh(ctx, scrollID)
		if err != nil {
			f.clearScroll(ctx, scrollID)
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

	f.clearScroll(ctx, scrollID)

	f.mu.Lock()
	f.cache = buffer
	f.mu.Unlock()
	f.cacheInitialized.Store(true)
	f.esCacheEntriesCount.Record(ctx, int64(len(f.cache)))
	f.last = time.Now()
	return nil
}

func (f *ElasticsearchFetcher) clearScroll(ctx context.Context, scrollID string) {
	resp, err := esapi.ClearScrollRequest{
		ScrollID: []string{scrollID},
	}.Do(ctx, f.client)
	if err != nil {
		f.logger.Warnf("failed to clear scroll: %v", err)
		return
	}

	if resp.IsError() {
		f.logger.Warnf("clearscroll request returned error: %s", resp.Status())
	}

	resp.Body.Close()
}

func (f *ElasticsearchFetcher) singlePageRefresh(ctx context.Context, scrollID string) (cacheResult, error) {
	var result cacheResult
	var err error
	var resp *esapi.Response

	switch scrollID {
	case "":
		resp, err = esapi.SearchRequest{
			Index:  []string{ElasticsearchIndexName},
			Size:   &f.searchSize,
			Scroll: f.cacheDuration,
		}.Do(ctx, f.client)
	default:
		resp, err = esapi.ScrollRequest{
			ScrollID: scrollID,
			Scroll:   f.cacheDuration,
		}.Do(ctx, f.client)
	}
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
