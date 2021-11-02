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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/elasticsearch"
	logs "github.com/elastic/apm-server/log"
)

func Test_esFetcher_fetchError(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode   int
		clientError  bool
		responseBody io.Reader
		temporary    bool
	}{
		"es not reachable": {
			clientError: true,
			temporary:   true,
		},
		"es bad request": {
			statusCode: http.StatusBadRequest,
		},
		"empty sourcemap string": {
			statusCode: http.StatusOK,
			responseBody: sourcemapSearchResponseBody(1, []map[string]interface{}{{
				"_source": map[string]interface{}{
					"sourcemap": map[string]interface{}{
						"sourcemap": "",
					},
				},
			}}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			var client elasticsearch.Client
			if tc.clientError {
				client = newUnavailableElasticsearchClient(t)
			} else {
				client = newMockElasticsearchClient(t, tc.statusCode, tc.responseBody)
			}

			consumer, err := testESFetcher(client).Fetch(context.Background(), "abc", "1.0", "/tmp")
			if tc.temporary {
				assert.Contains(t, err.Error(), errMsgESFailure)
			} else {
				assert.NotContains(t, err.Error(), errMsgESFailure)
			}
			assert.Empty(t, consumer)
		})
	}
}

func Test_esFetcher_fetch(t *testing.T) {
	for name, tc := range map[string]struct {
		statusCode   int
		responseBody io.Reader
		filePath     string
	}{
		"no sourcemap found": {
			statusCode:   http.StatusNotFound,
			responseBody: sourcemapSearchResponseBody(0, nil),
		},
		"sourcemap indicated but not found": {
			statusCode:   http.StatusOK,
			responseBody: sourcemapSearchResponseBody(1, []map[string]interface{}{}),
		},
		"valid sourcemap found": {
			statusCode:   http.StatusOK,
			responseBody: sourcemapSearchResponseBody(1, []map[string]interface{}{sourcemapHit(validSourcemap)}),
			filePath:     "bundle.js",
		},
	} {
		t.Run(name, func(t *testing.T) {
			client := newMockElasticsearchClient(t, tc.statusCode, tc.responseBody)
			sourcemapConsumer, err := testESFetcher(client).Fetch(context.Background(), "abc", "1.0", "/tmp")
			require.NoError(t, err)

			if tc.filePath == "" {
				assert.Nil(t, sourcemapConsumer)
			} else {
				assert.NotNil(t, sourcemapConsumer)
				assert.Equal(t, tc.filePath, sourcemapConsumer.File())
			}
		})
	}
}

func testESFetcher(client elasticsearch.Client) *esFetcher {
	return &esFetcher{client: client, index: "apm-sourcemap", logger: logp.NewLogger(logs.Sourcemap)}
}

func sourcemapSearchResponseBody(hitsTotal int, hits []map[string]interface{}) io.Reader {
	resultHits := map[string]interface{}{
		"total": map[string]interface{}{
			"value": hitsTotal,
		},
	}
	if hits != nil {
		resultHits["hits"] = hits
	}
	result := map[string]interface{}{"hits": resultHits}
	data, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}

func sourcemapHit(sourcemap string) map[string]interface{} {
	return map[string]interface{}{
		"_source": map[string]interface{}{
			"sourcemap": map[string]interface{}{
				"sourcemap": sourcemap,
			},
		},
	}
}

// newUnavailableElasticsearchClient returns an elasticsearch.Client configured
// to send requests to an invalid (unavailable) host.
func newUnavailableElasticsearchClient(t testing.TB) elasticsearch.Client {
	var transport roundTripperFunc = func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("client error")
	}
	cfg := elasticsearch.DefaultConfig()
	cfg.Hosts = []string{"testing.invalid"}
	cfg.MaxRetries = 1
	client, err := elasticsearch.NewClientParams(elasticsearch.ClientParams{Config: cfg, Transport: transport})
	require.NoError(t, err)
	return client
}

// newMockElasticsearchClient returns an elasticsearch.Clien configured to send
// requests to an httptest.Server that responds to source map search requests
// with the given status code and response body.
func newMockElasticsearchClient(t testing.TB, statusCode int, responseBody io.Reader) elasticsearch.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// validSourcemap is an example of a valid sourcemap for use in tests.
const validSourcemap = `{
    "version": 3,
    "sources": [
        "webpack:///bundle.js",
        "",
        "webpack:///./scripts/index.js",
        "webpack:///./index.html",
        "webpack:///./scripts/app.js"
    ],
    "names": [
        "modules",
        "__webpack_require__",
        "moduleId",
        "installedModules",
        "exports",
        "module",
        "id",
        "loaded",
        "call",
        "m",
        "c",
        "p",
        "foo",
        "console",
        "log",
        "foobar"
    ],
    "mappings": "CAAS,SAAUA,GCInB,QAAAC,GAAAC,GAGA,GAAAC,EAAAD,GACA,MAAAC,GAAAD,GAAAE,OAGA,IAAAC,GAAAF,EAAAD,IACAE,WACAE,GAAAJ,EACAK,QAAA,EAUA,OANAP,GAAAE,GAAAM,KAAAH,EAAAD,QAAAC,IAAAD,QAAAH,GAGAI,EAAAE,QAAA,EAGAF,EAAAD,QAvBA,GAAAD,KAqCA,OATAF,GAAAQ,EAAAT,EAGAC,EAAAS,EAAAP,EAGAF,EAAAU,EAAA,GAGAV,EAAA,KDMM,SAASI,EAAQD,EAASH,GE3ChCA,EAAA,GAEAA,EAAA,GAEAW,OFmDM,SAASP,EAAQD,EAASH,GGxDhCI,EAAAD,QAAAH,EAAAU,EAAA,cH8DM,SAASN,EAAQD,GI9DvB,QAAAQ,KACAC,QAAAC,IAAAC,QAGAH",
    "file": "bundle.js",
    "sourcesContent": [
        "/******/ (function(modules) { // webpackBootstrap\n/******/ \t// The module cache\n/******/ \tvar installedModules = {};\n/******/\n/******/ \t// The require function\n/******/ \tfunction __webpack_require__(moduleId) {\n/******/\n/******/ \t\t// Check if module is in cache\n/******/ \t\tif(installedModules[moduleId])\n/******/ \t\t\treturn installedModules[moduleId].exports;\n/******/\n/******/ \t\t// Create a new module (and put it into the cache)\n/******/ \t\tvar module = installedModules[moduleId] = {\n/******/ \t\t\texports: {},\n/******/ \t\t\tid: moduleId,\n/******/ \t\t\tloaded: false\n/******/ \t\t};\n/******/\n/******/ \t\t// Execute the module function\n/******/ \t\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\n/******/\n/******/ \t\t// Flag the module as loaded\n/******/ \t\tmodule.loaded = true;\n/******/\n/******/ \t\t// Return the exports of the module\n/******/ \t\treturn module.exports;\n/******/ \t}\n/******/\n/******/\n/******/ \t// expose the modules object (__webpack_modules__)\n/******/ \t__webpack_require__.m = modules;\n/******/\n/******/ \t// expose the module cache\n/******/ \t__webpack_require__.c = installedModules;\n/******/\n/******/ \t// __webpack_public_path__\n/******/ \t__webpack_require__.p = \"\";\n/******/\n/******/ \t// Load entry module and return exports\n/******/ \treturn __webpack_require__(0);\n/******/ })\n/************************************************************************/\n/******/ ([\n/* 0 */\n/***/ function(module, exports, __webpack_require__) {\n\n\t// Webpack\n\t__webpack_require__(1)\n\t\n\t__webpack_require__(2)\n\t\n\tfoo()\n\n\n/***/ },\n/* 1 */\n/***/ function(module, exports, __webpack_require__) {\n\n\tmodule.exports = __webpack_require__.p + \"index.html\"\n\n/***/ },\n/* 2 */\n/***/ function(module, exports) {\n\n\tfunction foo() {\n\t    console.log(foobar)\n\t}\n\t\n\tfoo()\n\n\n/***/ }\n/******/ ]);\n\n\n/** WEBPACK FOOTER **\n ** bundle.js\n **/",
        " \t// The module cache\n \tvar installedModules = {};\n\n \t// The require function\n \tfunction __webpack_require__(moduleId) {\n\n \t\t// Check if module is in cache\n \t\tif(installedModules[moduleId])\n \t\t\treturn installedModules[moduleId].exports;\n\n \t\t// Create a new module (and put it into the cache)\n \t\tvar module = installedModules[moduleId] = {\n \t\t\texports: {},\n \t\t\tid: moduleId,\n \t\t\tloaded: false\n \t\t};\n\n \t\t// Execute the module function\n \t\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\n\n \t\t// Flag the module as loaded\n \t\tmodule.loaded = true;\n\n \t\t// Return the exports of the module\n \t\treturn module.exports;\n \t}\n\n\n \t// expose the modules object (__webpack_modules__)\n \t__webpack_require__.m = modules;\n\n \t// expose the module cache\n \t__webpack_require__.c = installedModules;\n\n \t// __webpack_public_path__\n \t__webpack_require__.p = \"\";\n\n \t// Load entry module and return exports\n \treturn __webpack_require__(0);\n\n\n\n/** WEBPACK FOOTER **\n ** webpack/bootstrap 6002740481c9666b0d38\n **/",
        "// Webpack\nrequire('../index.html')\n\nrequire('./app')\n\nfoo()\n\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./scripts/index.js\n ** module id = 0\n ** module chunks = 0\n **/",
        "module.exports = __webpack_public_path__ + \"index.html\"\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./index.html\n ** module id = 1\n ** module chunks = 0\n **/",
        "function foo() {\n    console.log(foobar)\n}\n\nfoo()\n\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./scripts/app.js\n ** module id = 2\n ** module chunks = 0\n **/"
    ],
    "sourceRoot": ""
}`
