package otlp

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/r8"
	"github.com/elastic/apm-server/internal/sourcemap"
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
	key := sourcemap.Identifier{
		Name:    "foo",
		Version: "1.0.1",
		Path:    "android",
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
	esFetcher := r8.NewMapFetcher(client)
	cachingFetcher, err := NewMapCachingFetcher(esFetcher, 100)
	require.NoError(t, err)
	return cachingFetcher
}

// newMockElasticsearchClient returns an elasticsearch.Client configured to send
// requests to an httptest.Server that responds to source map search requests
// with the given status code and response body.
func newMockElasticsearchClient(t testing.TB, statusCode int, responseBody io.Reader) *elasticsearch.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(statusCode)
		if responseBody != nil {
			io.Copy(w, responseBody)
		}
	}))
	t.Cleanup(srv.Close)
	config := elasticsearch.DefaultConfig()
	config.Backoff.Init = time.Nanosecond
	config.Hosts = []string{srv.URL}
	client, err := elasticsearch.NewClient(config)
	require.NoError(t, err)
	return client
}

func sourcemapESResponseBody(s string) io.Reader {
	return strings.NewReader(encodeSourcemap(s))
}

func encodeSourcemap(sourcemap string) string {
	b := &bytes.Buffer{}

	z := zlib.NewWriter(b)
	z.Write([]byte(sourcemap))
	z.Close()

	return base64.StdEncoding.EncodeToString(b.Bytes())
}

// newUnavailableElasticsearchClient returns an elasticsearch.Client configured
// to send requests to an invalid (unavailable) host.
func newUnavailableElasticsearchClient(t testing.TB) *elasticsearch.Client {
	var transport roundTripperFunc = func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("client error")
	}
	cfg := elasticsearch.DefaultConfig()
	cfg.Hosts = []string{"testing.invalid"}
	cfg.MaxRetries = 1
	cfg.Backoff.Init = time.Nanosecond
	cfg.Backoff.Max = time.Nanosecond
	client, err := elasticsearch.NewClientParams(elasticsearch.ClientParams{Config: cfg, Transport: transport})
	require.NoError(t, err)
	return client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// validSourcemap is an example of a valid sourcemap for use in tests.
const validAndroidSourcemap = `# compiler: R8
# compiler_version: 3.2.47
# min_api: 26
# common_typos_disable
# {"id":"com.android.tools.r8.mapping","version":"2.0"}
# pg_map_id: 127b14c
# pg_map_hash: SHA-256 127b14c0be5dd1b55beee544a8d0e7c9414b432868ed8bc54ca5cc43cba12435
a1.TableInfo$ForeignKey$$ExternalSyntheticOutline0 -> a1.e:
# {"id":"sourceFile","fileName":"R8$$SyntheticClass"}`
