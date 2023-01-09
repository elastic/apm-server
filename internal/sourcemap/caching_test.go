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
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sourcemap/sourcemap"
	gocache "github.com/patrickmn/go-cache"
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
	_, err := NewCachingFetcher(nil, -1)
	require.Error(t, err)

	f, err := NewCachingFetcher(nil, 100)
	require.NoError(t, err)
	assert.NotNil(t, f.cache)
}

func TestStore_Fetch(t *testing.T) {
	serviceName, serviceVersion, path := "foo", "1.0.1", "/tmp"
	key := "foo_1.0.1_/tmp"

	t.Run("cache", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			var nilConsumer *sourcemap.Consumer
			store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK,
				sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit(validSourcemap)}),
			))
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
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusOK,
			sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit(validSourcemap)}),
		))
		mapper, err := store.Fetch(context.Background(), serviceName, serviceVersion, path)
		require.NoError(t, err)
		require.NotNil(t, mapper)

		// ensure sourcemap is added to cache
		cached, found := store.cache.Get(key)
		require.True(t, found)
		assert.Equal(t, mapper, cached)
	})

	t.Run("notFoundInES", func(t *testing.T) {
		store := testCachingFetcher(t, newMockElasticsearchClient(t, http.StatusNotFound, sourcemapSearchResponseBody(0, nil)))
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
			"invalid": newMockElasticsearchClient(t, http.StatusOK,
				sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit("foo")}),
			),
			"unsupportedVersion": newMockElasticsearchClient(t, http.StatusOK,
				sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit(unsupportedVersionSourcemap)}),
			),
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

func TestFetchContext(t *testing.T) {
	var (
		apikey  = "supersecret"
		name    = "webapp"
		version = "1.0.0"
		path    = "/my/path/to/bundle.js.map"
		c       = http.DefaultClient
	)

	requestReceived := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestReceived <- struct{}{}:
		case <-r.Context().Done():
			return
		}
		// block until the client cancels the request
		<-r.Context().Done()
	}))
	defer ts.Close()
	tsURL, _ := url.Parse(ts.URL)

	fleetServerURLs := []*url.URL{tsURL}
	fleetFetcher, err := NewFleetFetcher(c, apikey, fleetServerURLs, []FleetArtifactReference{{
		ServiceName:        name,
		ServiceVersion:     version,
		BundleFilepath:     path,
		FleetServerURLPath: "",
	}})
	assert.NoError(t, err)

	store, err := NewCachingFetcher(fleetFetcher, time.Minute)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetchReturned := make(chan error, 1)
	go func() {
		defer close(fetchReturned)
		_, err := store.Fetch(ctx, name, version, path)
		fetchReturned <- err
	}()
	select {
	case <-requestReceived:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for server to receive request")
	}

	// Check that cancelling the context unblocks the request.
	cancel()
	select {
	case err := <-fetchReturned:
		assert.True(t, errors.Is(err, context.Canceled))
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Fetch to return")
	}
}

func TestConcurrentFetch(t *testing.T) {
	for _, tc := range []struct {
		calledWant, errWant, succsWant int64
	}{
		{calledWant: 1, errWant: 0, succsWant: 10},
		{calledWant: 2, errWant: 1, succsWant: 9},
		{calledWant: 4, errWant: 3, succsWant: 7},
	} {
		var (
			called, errs, succs int64

			apikey  = "supersecret"
			name    = "webapp"
			version = "1.0.0"
			path    = "/my/path/to/bundle.js.map"
			c       = http.DefaultClient
			res     = fmt.Sprintf(`{"sourceMap":%s}`, validSourcemap)

			errsLeft = tc.errWant
		)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&called, 1)
			// Simulate the wait for a network request.
			time.Sleep(50 * time.Millisecond)
			if errsLeft > 0 {
				errsLeft--
				http.Error(w, "err", http.StatusInternalServerError)
				return
			}
			wr := zlib.NewWriter(w)
			defer wr.Close()
			wr.Write([]byte(res))
		}))
		defer ts.Close()
		tsURL, _ := url.Parse(ts.URL)

		fleetServerURLs := []*url.URL{tsURL}
		fleetFetcher, err := NewFleetFetcher(c, apikey, fleetServerURLs, []FleetArtifactReference{{
			ServiceName:        name,
			ServiceVersion:     version,
			BundleFilepath:     path,
			FleetServerURLPath: "",
		}})
		assert.NoError(t, err)

		fetcher, err := NewCachingFetcher(fleetFetcher, time.Minute)
		assert.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < int(tc.succsWant+tc.errWant); i++ {
			wg.Add(1)
			go func() {
				consumer, err := fetcher.Fetch(context.Background(), name, version, path)
				if err != nil {
					atomic.AddInt64(&errs, 1)
				} else {
					assert.NotNil(t, consumer)
					atomic.AddInt64(&succs, 1)
				}

				wg.Done()
			}()
		}

		wg.Wait()
		assert.Equal(t, tc.errWant, errs)
		assert.Equal(t, tc.calledWant, called)
		assert.Equal(t, tc.succsWant, succs)
	}
}

func TestExpiration(t *testing.T) {
	store := testCachingFetcher(t, newUnavailableElasticsearchClient(t)) //if ES was queried it would return an error
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

func testCachingFetcher(t *testing.T, client *elasticsearch.Client) *CachingFetcher {
	esFetcher := NewElasticsearchFetcher(client, "apm-*sourcemap*")
	cachingFetcher, err := NewCachingFetcher(esFetcher, time.Minute)
	require.NoError(t, err)
	return cachingFetcher
}
