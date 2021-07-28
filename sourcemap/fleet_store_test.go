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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/elastic/apm-server/beater/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFleetFetch(t *testing.T) {
	var (
		hasAuth       bool
		apikey        = "supersecret"
		name          = "webapp"
		version       = "1.0.0"
		path          = "/my/path/to/bundle.js.map"
		c             = http.DefaultClient
		sourceMapPath = "/api/fleet/artifact"
	)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, sourceMapPath, r.URL.Path)
		auth := r.Header.Get("Authorization")
		hasAuth = auth == "ApiKey "+apikey
		// zlib compress
		wr := zlib.NewWriter(w)
		defer wr.Close()
		wr.Write([]byte(resp))
	})

	ts0 := httptest.NewServer(h)
	defer ts0.Close()

	ts1 := httptest.NewServer(h)
	defer ts1.Close()

	fleetCfg := &config.Fleet{
		Hosts:        []string{ts0.URL[7:], ts1.URL[7:]},
		Protocol:     "http",
		AccessAPIKey: apikey,
		TLS:          nil,
	}

	cfgs := []config.SourceMapMetadata{
		{
			ServiceName:    name,
			ServiceVersion: version,
			BundleFilepath: path,
			SourceMapURL:   sourceMapPath,
		},
	}
	fb, err := newFleetStore(c, fleetCfg, cfgs)
	assert.NoError(t, err)

	gotRes, err := fb.fetch(context.Background(), name, version, path)
	require.NoError(t, err)

	assert.Contains(t, gotRes, "webpack:///bundle.js")
	assert.True(t, hasAuth)
}

func TestMultipleFleetHostsQueryFailureFetch(t *testing.T) {
	var (
		requestCount  int32
		apikey        = "supersecret"
		name          = "webapp"
		version       = "1.0.0"
		path          = "/my/path/to/bundle.js.map"
		c             = http.DefaultClient
		sourceMapPath = "/api/fleet/artifact"
	)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "err", http.StatusInternalServerError)
		atomic.AddInt32(&requestCount, 1)
	})

	ts0 := httptest.NewServer(h)
	defer ts0.Close()

	ts1 := httptest.NewServer(h)
	defer ts1.Close()

	ts2 := httptest.NewServer(h)
	defer ts2.Close()

	fleetCfg := &config.Fleet{
		Hosts:        []string{ts0.URL[7:], ts1.URL[7:], ts2.URL[7:]},
		Protocol:     "http",
		AccessAPIKey: apikey,
		TLS:          nil,
	}

	cfgs := []config.SourceMapMetadata{
		{
			ServiceName:    name,
			ServiceVersion: version,
			BundleFilepath: path,
			SourceMapURL:   sourceMapPath,
		},
	}
	f, err := newFleetStore(c, fleetCfg, cfgs)
	assert.NoError(t, err)

	resp, err := f.fetch(context.Background(), name, version, path)
	assert.EqualValues(t, len(fleetCfg.Hosts), requestCount)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMsgFleetFailure)
	assert.Equal(t, "", resp)
}

var resp = "{\"serviceName\":\"web-app\",\"serviceVersion\":\"1.0.0\",\"bundleFilepath\":\"/test/e2e/general-usecase/bundle.js.map\",\"sourceMap\":{\"version\":3,\"sources\":[\"webpack:///bundle.js\",\"\",\"webpack:///./scripts/index.js\",\"webpack:///./index.html\",\"webpack:///./scripts/app.js\"],\"names\":[\"modules\",\"__webpack_require__\",\"moduleId\",\"installedModules\",\"exports\",\"module\",\"id\",\"loaded\",\"call\",\"m\",\"c\",\"p\",\"foo\",\"console\",\"log\",\"foobar\"],\"mappings\":\"CAAS,SAAUA,GCInB,QAAAC,GAAAC,GAGA,GAAAC,EAAAD,GACA,MAAAC,GAAAD,GAAAE,OAGA,IAAAC,GAAAF,EAAAD,IACAE,WACAE,GAAAJ,EACAK,QAAA,EAUA,OANAP,GAAAE,GAAAM,KAAAH,EAAAD,QAAAC,IAAAD,QAAAH,GAGAI,EAAAE,QAAA,EAGAF,EAAAD,QAvBA,GAAAD,KAqCA,OATAF,GAAAQ,EAAAT,EAGAC,EAAAS,EAAAP,EAGAF,EAAAU,EAAA,GAGAV,EAAA,KDMM,SAASI,EAAQD,EAASH,GE3ChCA,EAAA,GAEAA,EAAA,GAEAW,OFmDM,SAASP,EAAQD,EAASH,GGxDhCI,EAAAD,QAAAH,EAAAU,EAAA,cH8DM,SAASN,EAAQD,GI9DvB,QAAAQ,KACAC,QAAAC,IAAAC,QAGAH\",\"file\":\"bundle.js\",\"sourcesContent\":[\"/******/ (function(modules) { // webpackBootstrap\\n/******/ \\t// The module cache\\n/******/ \\tvar installedModules = {};\\n/******/\\n/******/ \\t// The require function\\n/******/ \\tfunction __webpack_require__(moduleId) {\\n/******/\\n/******/ \\t\\t// Check if module is in cache\\n/******/ \\t\\tif(installedModules[moduleId])\\n/******/ \\t\\t\\treturn installedModules[moduleId].exports;\\n/******/\\n/******/ \\t\\t// Create a new module (and put it into the cache)\\n/******/ \\t\\tvar module = installedModules[moduleId] = {\\n/******/ \\t\\t\\texports: {},\\n/******/ \\t\\t\\tid: moduleId,\\n/******/ \\t\\t\\tloaded: false\\n/******/ \\t\\t};\\n/******/\\n/******/ \\t\\t// Execute the module function\\n/******/ \\t\\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\\n/******/\\n/******/ \\t\\t// Flag the module as loaded\\n/******/ \\t\\tmodule.loaded = true;\\n/******/\\n/******/ \\t\\t// Return the exports of the module\\n/******/ \\t\\treturn module.exports;\\n/******/ \\t}\\n/******/\\n/******/\\n/******/ \\t// expose the modules object (__webpack_modules__)\\n/******/ \\t__webpack_require__.m = modules;\\n/******/\\n/******/ \\t// expose the module cache\\n/******/ \\t__webpack_require__.c = installedModules;\\n/******/\\n/******/ \\t// __webpack_public_path__\\n/******/ \\t__webpack_require__.p = \\\"\\\";\\n/******/\\n/******/ \\t// Load entry module and return exports\\n/******/ \\treturn __webpack_require__(0);\\n/******/ })\\n/************************************************************************/\\n/******/ ([\\n/* 0 */\\n/***/ function(module, exports, __webpack_require__) {\\n\\n\\t// Webpack\\n\\t__webpack_require__(1)\\n\\t\\n\\t__webpack_require__(2)\\n\\t\\n\\tfoo()\\n\\n\\n/***/ },\\n/* 1 */\\n/***/ function(module, exports, __webpack_require__) {\\n\\n\\tmodule.exports = __webpack_require__.p + \\\"index.html\\\"\\n\\n/***/ },\\n/* 2 */\\n/***/ function(module, exports) {\\n\\n\\tfunction foo() {\\n\\t    console.log(foobar)\\n\\t}\\n\\t\\n\\tfoo()\\n\\n\\n/***/ }\\n/******/ ]);\\n\\n\\n/** WEBPACK FOOTER **\\n ** bundle.js\\n **/\",\" \\t// The module cache\\n \\tvar installedModules = {};\\n\\n \\t// The require function\\n \\tfunction __webpack_require__(moduleId) {\\n\\n \\t\\t// Check if module is in cache\\n \\t\\tif(installedModules[moduleId])\\n \\t\\t\\treturn installedModules[moduleId].exports;\\n\\n \\t\\t// Create a new module (and put it into the cache)\\n \\t\\tvar module = installedModules[moduleId] = {\\n \\t\\t\\texports: {},\\n \\t\\t\\tid: moduleId,\\n \\t\\t\\tloaded: false\\n \\t\\t};\\n\\n \\t\\t// Execute the module function\\n \\t\\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\\n\\n \\t\\t// Flag the module as loaded\\n \\t\\tmodule.loaded = true;\\n\\n \\t\\t// Return the exports of the module\\n \\t\\treturn module.exports;\\n \\t}\\n\\n\\n \\t// expose the modules object (__webpack_modules__)\\n \\t__webpack_require__.m = modules;\\n\\n \\t// expose the module cache\\n \\t__webpack_require__.c = installedModules;\\n\\n \\t// __webpack_public_path__\\n \\t__webpack_require__.p = \\\"\\\";\\n\\n \\t// Load entry module and return exports\\n \\treturn __webpack_require__(0);\\n\\n\\n\\n/** WEBPACK FOOTER **\\n ** webpack/bootstrap 6002740481c9666b0d38\\n **/\",\"// Webpack\\nrequire('../index.html')\\n\\nrequire('./app')\\n\\nfoo()\\n\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./scripts/index.js\\n ** module id = 0\\n ** module chunks = 0\\n **/\",\"module.exports = __webpack_public_path__ + \\\"index.html\\\"\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./index.html\\n ** module id = 1\\n ** module chunks = 0\\n **/\",\"function foo() {\\n    console.log(foobar)\\n}\\n\\nfoo()\\n\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./scripts/app.js\\n ** module id = 2\\n ** module chunks = 0\\n **/\"],\"sourceRoot\":\"\"}}"
