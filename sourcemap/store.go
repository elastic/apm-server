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
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
)

const (
	minCleanupIntervalSeconds float64 = 60
)

var (
	errInit = errors.New("Cache cannot be initialized. Expiration and CleanupInterval need to be >= 0")
)

// Store holds information necessary to fetch a sourcemap, either from an Elasticsearch instance or an internal cache.
type Store struct {
	cache   *gocache.Cache
	backend backend
	logger  *logp.Logger
}

type backend interface {
	fetch(ctx context.Context, name, version, path string) (string, error)
}

// NewStore creates a new instance for fetching sourcemaps based on the
// provided config.SourceMapping. Only one of source_mapping.elasticsearch or
// sourcemapping.source_maps can be configured.
func NewStore(beatInfo beat.Info, cfg config.SourceMapping) (*Store, error) {
	// We're already checking for this in config parsing, so do we need it
	// here a second time?
	if cfg.ESConfig != nil && len(cfg.SourceMapConfigs) > 0 {
		return nil, errors.New("configuring both source_mapping.elasticsearch and sourcemapping.source_maps not allowed")
	}
	expiration := cfg.Cache.Expiration
	if expiration < 0 {
		return nil, errInit
	}

	var b backend
	logger := logp.NewLogger(logs.Sourcemap)

	if len(cfg.SourceMapConfigs) == 0 {
		esClient, err := elasticsearch.NewClient(cfg.ESConfig)
		if err != nil {
			return nil, err
		}
		index := strings.ReplaceAll(cfg.IndexPattern, "%{[observer.version]}", beatInfo.Version)
		b = &esStore{client: esClient, index: index, logger: logger}
	} else {
		b = newFleetBackend(cfg.ESConfig.APIKey, cfg.SourceMapConfigs, http.DefaultClient)
	}

	return &Store{
		cache:   gocache.New(expiration, cleanupInterval(expiration)),
		backend: b,
		logger:  logger,
	}, nil
}

// Fetch a sourcemap from the store.
func (s *Store) Fetch(ctx context.Context, name string, version string, path string) (*sourcemap.Consumer, error) {
	key := key([]string{name, version, path})

	// fetch from cache
	if val, found := s.cache.Get(key); found {
		consumer, _ := val.(*sourcemap.Consumer)
		return consumer, nil
	}

	// fetch from Elasticsearch and ensure caching for all non-temporary results
	sourcemapStr, err := s.backend.fetch(ctx, name, version, path)
	if err != nil {
		// TODO: Check for errors from both stores
		if !strings.Contains(err.Error(), errMsgESFailure) {
			s.add(key, nil)
		}
		return nil, err
	}

	if sourcemapStr == emptyResult {
		s.add(key, nil)
		return nil, nil
	}

	consumer, err := sourcemap.Parse("", []byte(sourcemapStr))
	if err != nil {
		s.add(key, nil)
		return nil, errors.Wrap(err, errMsgParseSourcemap)
	}
	s.add(key, consumer)
	return consumer, nil
}

// Added ensures the internal cache is cleared for the given parameters. This should be called when a sourcemap is uploaded.
func (s *Store) Added(ctx context.Context, name string, version string, path string) {
	if sourcemap, err := s.Fetch(ctx, name, version, path); err == nil && sourcemap != nil {
		s.logger.Warnf("Overriding sourcemap for service %s version %s and file %s",
			name, version, path)
	}
	key := key([]string{name, version, path})
	s.cache.Delete(key)
	if !s.logger.IsDebug() {
		return
	}
	s.logger.Debugf("Removed id %v. Cache now has %v entries.", key, s.cache.ItemCount())
}

func (s *Store) add(key string, consumer *sourcemap.Consumer) {
	s.cache.SetDefault(key, consumer)
	if !s.logger.IsDebug() {
		return
	}
	s.logger.Debugf("Added id %v. Cache now has %v entries.", key, s.cache.ItemCount())
}

func key(s []string) string {
	return strings.Join(s, "_")
}

func cleanupInterval(ttl time.Duration) time.Duration {
	return time.Duration(math.Max(ttl.Seconds(), minCleanupIntervalSeconds)) * time.Second
}
