package r8

import (
	"context"
	"net/http"
	"testing"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewCachingFetcher(t *testing.T) {
	_, err := NewMapCachingFetcher(nil, -1)
	require.Error(t, err)

	f, err := NewMapCachingFetcher(nil, 100)
	require.NoError(t, err)
	assert.NotNil(t, f.cache)
}

func TestStore_Fetch(t *testing.T) {
	serviceName, serviceVersion := "foo", "1.0.1"
	key := identifier{
		Name:    "foo",
		Version: "1.0.1",
	}

	t.Run("cache", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(validAndroidSourcemap)))
			store.add(key, nil)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion)
			assert.Nil(t, mapper)
			assert.Nil(t, err)
		})

		t.Run("sourcemapData", func(t *testing.T) {
			data := []byte{'a'}
			store := testCachingFetcher(t, newUnavailableElasticsearchClient(t))
			store.add(key, data)

			mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion)
			require.NoError(t, err)
			assert.Equal(t, data, mapper)

		})
	})

	t.Run("validFromES", func(t *testing.T) {
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody(validAndroidSourcemap)))
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion)
		require.NoError(t, err)
		require.NotNil(t, mapper)

		// ensure sourcemap is added to cache
		cached, found := store.cache.Get(key)
		require.True(t, found)
		assert.Equal(t, mapper, cached)
	})

	t.Run("notFoundInES", func(t *testing.T) {
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK, sourcemapESResponseBody("")))
		//not cached
		cached, found := store.cache.Get(key)
		require.False(t, found)
		require.Nil(t, cached)

		//fetch nil value, leading to error
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion)
		require.Nil(t, err)
		require.Nil(t, mapper)

		// ensure nil value is added to cache
		cached, found = store.cache.Get(key)
		assert.True(t, found)
		assert.Nil(t, cached)
	})

	t.Run("noConnectionToES", func(t *testing.T) {
		store := testCachingFetcher(t, newUnavailableElasticsearchClient(t))
		//not cached
		_, found := store.cache.Get(key)
		require.False(t, found)

		//fetch nil value, leading to error
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion)
		require.Error(t, err)
		require.Nil(t, mapper)

		// ensure not cached
		_, found = store.cache.Get(key)
		assert.False(t, found)
	})
}

func testCachingFetcher(t *testing.T, client *elasticsearch.Client) *MapCachingFetcher {
	esFetcher := NewMapFetcher(client)
	cachingFetcher, err := NewMapCachingFetcher(esFetcher, 100)
	require.NoError(t, err)
	return cachingFetcher
}
