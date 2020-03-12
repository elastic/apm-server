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
	"strings"
	"time"

	"github.com/elastic/apm-server/elasticsearch"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"

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
	esStore *esStore
	logger  *logp.Logger
}

// NewStore creates a new instance for fetching sourcemaps. The client and index parameters are needed to be able to
// fetch sourcemaps from Elasticsearch. The expiration time is used for the internal cache.
func NewStore(client elasticsearch.Client, index string, expiration time.Duration) (*Store, error) {
	if expiration < 0 {
		return nil, errInit
	}
	logger := logp.NewLogger(logs.Sourcemap)
	return &Store{
		cache:   gocache.New(expiration, cleanupInterval(expiration)),
		esStore: &esStore{client: client, index: index, logger: logger},
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
	sourcemapStr, err := s.esStore.fetch(ctx, name, version, path)
	if err != nil {
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
func (s *Store) Added(name string, version string, path string) {
	if sourcemap, err := s.Fetch(context.TODO(), name, version, path); err == nil && sourcemap != nil {
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
