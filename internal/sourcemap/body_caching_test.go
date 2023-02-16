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
	"net/http"
	"testing"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/elasticsearch"
)

var unsupportedVersionSourcemap = `{
  "version": 1,
  "sources": ["webpack:///bundle.js"],
  "names": [],
  "mappings": "CAAS",
  "file": "bundle.js",
  "sourcesContent": [],
  "sourceRoot": ""
}`

func Test_NewCachingFetcher(t *testing.T) {
	// pass a closed channel to aovid leaking a goroutine
	// by listening to a nil channel
	ch := make(chan []identifier)
	close(ch)

	_, err := NewBodyCachingFetcher(nil, -1, ch)
	require.Error(t, err)

	f, err := NewBodyCachingFetcher(nil, 100, ch)
	require.NoError(t, err)
	assert.NotNil(t, f.cache)
}

func TestStore_Fetch(t *testing.T) {
	serviceName, serviceVersion, path := "foo", "1.0.1", "/tmp"
	key := identifier{
		name:    "foo",
		version: "1.0.1",
		path:    "/tmp",
	}

	t.Run("cache", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			var nilConsumer *sourcemap.Consumer
			store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(true, validSourcemap)))
			store.add(key, nilConsumer)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
			assert.Nil(t, mapper)
			assert.Nil(t, err)
		})

		t.Run("sourcemapConsumer", func(t *testing.T) {
			consumer := &sourcemap.Consumer{}
			store := testCachingFetcher(t, newUnavailableElasticsearchClient(t))
			store.add(key, consumer)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
			require.NoError(t, err)
			assert.Equal(t, consumer, mapper)

		})
	})

	t.Run("validFromES", func(t *testing.T) {
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(true, validSourcemap)))
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
		require.NoError(t, err)
		require.NotNil(t, mapper)

		// ensure sourcemap is added to cache
		cached, found := store.cache.Get(key)
		require.True(t, found)
		assert.Equal(t, mapper, cached)
	})

	t.Run("notFoundInES", func(t *testing.T) {
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(false, "")))
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
		for name, client := range map[string]*elasticsearch.Client{
			"invalid":            newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(true, "foo")),
			"unsupportedVersion": newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(true, unsupportedVersionSourcemap)),
		} {
			t.Run(name, func(t *testing.T) {
				store := testCachingFetcher(t, client)
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
		store := testCachingFetcher(t, newUnavailableElasticsearchClient(t))
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

func testCachingFetcher(t *testing.T, client *elasticsearch.Client) *BodyCachingFetcher {
	ch := make(chan []identifier)
	close(ch)

	esFetcher := NewElasticsearchFetcher(client, "apm-*sourcemap*")
	cachingFetcher, err := NewBodyCachingFetcher(esFetcher, 100, ch)
	require.NoError(t, err)
	return cachingFetcher
}
