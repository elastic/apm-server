package r8

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
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MapFetcher_fetchError(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode         int
		clientError        bool
		responseBody       io.Reader
		temporary          bool
		expectedErrMessage string
	}{
		"es not reachable": {
			clientError:        true,
			temporary:          true,
			expectedErrMessage: "failure querying ES: client error",
		},
		"es bad request": {
			statusCode:         http.StatusBadRequest,
			expectedErrMessage: "ES returned unknown status code: 400 Bad Request",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var client *elasticsearch.Client
			if tc.clientError {
				client = newUnavailableElasticsearchClient(t)
			} else {
				client = newMockElasticsearchClient(t, tc.statusCode, tc.responseBody)
			}

			consumer, err := testMapFetcher(client).Fetch(context.Background(), "abc", "1.0")
			assert.Equal(t, tc.expectedErrMessage, err.Error())
			assert.Empty(t, consumer)
		})
	}
}

func Test_MapFetcher_fetch(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode   int
		responseBody io.Reader
		filePath     string
	}{
		"no android sourcemap found": {
			statusCode:   http.StatusOK,
			responseBody: sourcemapESResponseBody(""),
		},
		"valid android sourcemap found": {
			statusCode:   http.StatusOK,
			responseBody: sourcemapESResponseBody(validAndroidSourcemap),
			filePath:     "android",
		},
	} {
		t.Run(name, func(t *testing.T) {
			client := newMockElasticsearchClient(t, tc.statusCode, tc.responseBody)
			androidSourceMapReader, err := testMapFetcher(client).Fetch(context.Background(), "abc", "1.0")
			require.NoError(t, err)

			if tc.filePath == "" {
				assert.Nil(t, androidSourceMapReader)
			} else {
				assert.NotNil(t, androidSourceMapReader)
				assert.Equal(t, []byte(validAndroidSourcemap), androidSourceMapReader)
			}
		})
	}
}
func testMapFetcher(client *elasticsearch.Client) *MapFetcher {
	return &MapFetcher{client: client, logger: logp.NewLogger(logs.Sourcemap)}
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

// newMockElasticsearchClient returns an elasticsearch.Clien configured to send
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
