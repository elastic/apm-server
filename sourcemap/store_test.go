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
	"fmt"
	"testing"
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/sourcemap/test"
)

func Test_NewStore(t *testing.T) {
	logger := logp.NewLogger(logs.Sourcemap)

	_, err := NewStore(nil, logger, -1)
	require.Error(t, err)

	f, err := NewStore(nil, logger, 100)
	require.NoError(t, err)
	assert.NotNil(t, f.cache)
}

func TestStore_Fetch(t *testing.T) {
	serviceName, serviceVersion, path := "foo", "1.0.1", "/tmp"
	key := "foo_1.0.1_/tmp"

	t.Run("cache", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			var nilConsumer *sourcemap.Consumer
			store := testStore(t, test.ESClientWithValidSourcemap(t)) //if ES was queried, it would return a valid sourcemap
			store.add(key, nilConsumer)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
			assert.Nil(t, mapper)
			assert.Nil(t, err)
		})

		t.Run("sourcemapConsumer", func(t *testing.T) {
			consumer := &sourcemap.Consumer{}
			store := testStore(t, test.ESClientUnavailable(t)) //if ES was queried, it would return a server error
			store.add(key, consumer)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
			require.NoError(t, err)
			assert.Equal(t, consumer, mapper)

		})
	})

	t.Run("validFromES", func(t *testing.T) {
		store := testStore(t, test.ESClientWithValidSourcemap(t))
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
		require.NoError(t, err)
		require.NotNil(t, mapper)

		// ensure sourcemap is added to cache
		cached, found := store.cache.Get(key)
		require.True(t, found)
		assert.Equal(t, mapper, cached)
	})

	t.Run("notFoundInES", func(t *testing.T) {

		store := testStore(t, test.ESClientWithSourcemapNotFound(t))
		//not cached
		cached, found := store.cache.Get(key)
		require.False(t, found)
		require.Nil(t, cached)

		//fetch nil value, leading to error
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
		require.Nil(t, err)
		require.Nil(t, mapper)

		// ensure nil value is added to cache
		cached, found = store.cache.Get(key)
		assert.True(t, found)
		assert.Nil(t, cached)
	})

	t.Run("invalidFromES", func(t *testing.T) {
		for name, client := range map[string]elasticsearch.Client{
			"invalid":            test.ESClientWithInvalidSourcemap(t),
			"unsupportedVersion": test.ESClientWithUnsupportedSourcemap(t),
		} {
			t.Run(name, func(t *testing.T) {
				store := testStore(t, client)
				//not cached
				cached, found := store.cache.Get(key)
				require.False(t, found)
				require.Nil(t, cached)

				//fetch nil value, leading to error
				mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
				require.Error(t, err)
				require.Nil(t, mapper)

				// ensure nil value is added to cache
				cached, found = store.cache.Get(key)
				assert.True(t, found)
				assert.Nil(t, cached)
			})
		}
	})

	t.Run("noConnectionToES", func(t *testing.T) {
		store := testStore(t, test.ESClientUnavailable(t))
		//not cached
		_, found := store.cache.Get(key)
		require.False(t, found)

		//fetch nil value, leading to error
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
		require.Error(t, err)
		require.Nil(t, mapper)

		// ensure not cached
		_, found = store.cache.Get(key)
		assert.False(t, found)
	})
}

func TestStore_Added(t *testing.T) {
	name, version, path := "foo", "1.0.1", "/tmp"
	key := "foo_1.0.1_/tmp"

	// setup
	// remove empty sourcemap from cache, and valid one with File() == "bundle.js" from Elasticsearch
	store := testStore(t, test.ESClientWithValidSourcemap(t))
	store.add(key, &sourcemap.Consumer{})

	mapper, err := store.Fetch(context.Background(), name, version, path)
	require.NoError(t, err)
	assert.Equal(t, &sourcemap.Consumer{}, mapper)
	assert.Equal(t, "", mapper.File())

	// remove from cache, afterwards sourcemap should be fetched from ES
	store.Added(context.Background(), name, version, path)
	mapper, err = store.Fetch(context.Background(), name, version, path)
	require.NoError(t, err)
	assert.NotNil(t, &sourcemap.Consumer{}, mapper)
	assert.Equal(t, "bundle.js", mapper.File())
}

func TestExpiration(t *testing.T) {
	store := testStore(t, test.ESClientUnavailable(t)) //if ES was queried it would return an error
	store.cache = gocache.New(25*time.Millisecond, 100)
	store.add("foo_1.0.1_/tmp", &sourcemap.Consumer{})
	name, version, path := "foo", "1.0.1", "/tmp"

	// sourcemap is cached
	mapper, err := store.Fetch(context.Background(), name, version, path)
	require.NoError(t, err)
	assert.Equal(t, &sourcemap.Consumer{}, mapper)

	time.Sleep(25 * time.Millisecond)
	// cache is cleared, sourcemap is fetched from ES leading to an error
	mapper, err = store.Fetch(context.Background(), name, version, path)
	require.Error(t, err)
	assert.Nil(t, mapper)
}

func TestCleanupInterval(t *testing.T) {
	tests := []struct {
		ttl      time.Duration
		expected float64
	}{
		{expected: 1},
		{ttl: 30 * time.Second, expected: 1},
		{ttl: 30 * time.Second, expected: 1},
		{ttl: 60 * time.Second, expected: 1},
		{ttl: 61 * time.Second, expected: 61.0 / 60},
		{ttl: 5 * time.Minute, expected: 5},
	}
	for idx, test := range tests {
		out := cleanupInterval(test.ttl)
		assert.Equal(t, test.expected, out.Minutes(),
			fmt.Sprintf("(%v) expected %v minutes, received %v minutes", idx, test.expected, out.Minutes()))
	}
}

func testStore(t *testing.T, client elasticsearch.Client) *Store {
	logger := logp.NewLogger(logs.Sourcemap)
	b := NewESStore(client, "apm-*sourcemap*", logger)

	store, err := NewStore(b, logger, time.Minute)
	require.NoError(t, err)
	return store
}
