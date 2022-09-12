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
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-sourcemap/sourcemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFleetFetch(t *testing.T) {
	var (
		apikey        = "supersecret"
		name          = "webapp"
		version       = "1.0.0"
		path1         = "/my/path/to/bundle1.js.map"
		path2         = "/my/path/to/bundle2.js.map"
		c             = http.DefaultClient
		sourceMapPath = "/api/fleet/artifact"
	)

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, sourceMapPath, r.URL.Path)
		if auth := r.Header.Get("Authorization"); auth != "ApiKey "+apikey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// zlib compress
		wr := zlib.NewWriter(w)
		defer wr.Close()
		wr.Write([]byte(resp))
	})

	ts0 := httptest.NewServer(h)
	defer ts0.Close()
	ts0URL, _ := url.Parse(ts0.URL)

	ts1 := httptest.NewServer(h)
	defer ts1.Close()
	ts1URL, _ := url.Parse(ts1.URL)

	fleetServerURLs := []*url.URL{ts0URL, ts1URL}
	f, err := NewFleetFetcher(c, apikey, fleetServerURLs, []FleetArtifactReference{{
		ServiceName:        name,
		ServiceVersion:     version,
		BundleFilepath:     path1,
		FleetServerURLPath: sourceMapPath,
	}, {
		ServiceName:        name,
		ServiceVersion:     version,
		BundleFilepath:     "http://not_testing.invalid" + path2,
		FleetServerURLPath: sourceMapPath,
	}})
	assert.NoError(t, err)

	for _, path := range []string{path1, path2} {
		consumer, err := f.Fetch(context.Background(), name, version, path)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)

		// Check that only the URL *path* is used, and matches entries in the
		// fetcher stored with either absolute or relative URLs.
		consumer, err = f.Fetch(context.Background(), name, version, "http://testing.invalid"+path)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
	}
}

func TestFailedAndSuccessfulFleetHostsFetch(t *testing.T) {
	type response struct {
		consumer *sourcemap.Consumer
		err      error
	}
	var (
		apikey        = "supersecret"
		name          = "webapp"
		version       = "1.0.0"
		path          = "/my/path/to/bundle.js.map"
		c             = http.DefaultClient
		sourceMapPath = "/api/fleet/artifact"
		successc      = make(chan struct{})
		errc          = make(chan struct{})
		lastc         = make(chan struct{})
		waitc         = make(chan struct{})
		resc          = make(chan response)
		wg            sync.WaitGroup
	)
	wg.Add(3)
	defer func() {
		close(successc)
		close(errc)
		close(lastc)
		close(resc)
	}()

	hError := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
		errc <- struct{}{}
		http.Error(w, "err", http.StatusInternalServerError)
	})
	ts0 := httptest.NewServer(hError)
	ts0URL, _ := url.Parse(ts0.URL)

	h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
		successc <- struct{}{}
		wr := zlib.NewWriter(w)
		defer wr.Close()
		wr.Write([]byte(resp))
	})
	ts1 := httptest.NewServer(h1)
	ts1URL, _ := url.Parse(ts1.URL)

	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
		lastc <- struct{}{}
		close(waitc)
	})
	ts2 := httptest.NewServer(h2)
	ts2URL, _ := url.Parse(ts2.URL)

	fleetServerURLs := []*url.URL{ts0URL, ts1URL, ts2URL}
	f, err := NewFleetFetcher(c, apikey, fleetServerURLs, []FleetArtifactReference{{
		ServiceName:        name,
		ServiceVersion:     version,
		BundleFilepath:     path,
		FleetServerURLPath: sourceMapPath,
	}})
	assert.NoError(t, err)

	go func() {
		consumer, err := f.Fetch(context.Background(), name, version, path)
		resc <- response{consumer, err}
	}()
	// Make sure every server has received a request
	wg.Wait()
	<-errc
	<-successc
	res := <-resc
	assert.NoError(t, res.err)
	assert.NotNil(t, res.consumer)

	// Wait for h2
	<-lastc
	<-waitc
}

func TestAllFailedFleetHostsFetch(t *testing.T) {
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
	ts0URL, _ := url.Parse(ts0.URL)

	ts1 := httptest.NewServer(h)
	defer ts1.Close()
	ts1URL, _ := url.Parse(ts1.URL)

	ts2 := httptest.NewServer(h)
	defer ts2.Close()
	ts2URL, _ := url.Parse(ts2.URL)

	fleetServerURLs := []*url.URL{ts0URL, ts1URL, ts2URL}
	f, err := NewFleetFetcher(c, apikey, fleetServerURLs, []FleetArtifactReference{{
		ServiceName:        name,
		ServiceVersion:     version,
		BundleFilepath:     path,
		FleetServerURLPath: sourceMapPath,
	}})
	assert.NoError(t, err)

	consumer, err := f.Fetch(context.Background(), name, version, path)
	assert.EqualValues(t, len(fleetServerURLs), requestCount)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMsgFleetFailure)
	assert.Nil(t, consumer)
}

var resp = "{\"serviceName\":\"web-app\",\"serviceVersion\":\"1.0.0\",\"bundleFilepath\":\"/test/e2e/general-usecase/bundle.js.map\",\"sourceMap\":{\"version\":3,\"sources\":[\"webpack:///bundle.js\",\"\",\"webpack:///./scripts/index.js\",\"webpack:///./index.html\",\"webpack:///./scripts/app.js\"],\"names\":[\"modules\",\"__webpack_require__\",\"moduleId\",\"installedModules\",\"exports\",\"module\",\"id\",\"loaded\",\"call\",\"m\",\"c\",\"p\",\"foo\",\"console\",\"log\",\"foobar\"],\"mappings\":\"CAAS,SAAUA,GCInB,QAAAC,GAAAC,GAGA,GAAAC,EAAAD,GACA,MAAAC,GAAAD,GAAAE,OAGA,IAAAC,GAAAF,EAAAD,IACAE,WACAE,GAAAJ,EACAK,QAAA,EAUA,OANAP,GAAAE,GAAAM,KAAAH,EAAAD,QAAAC,IAAAD,QAAAH,GAGAI,EAAAE,QAAA,EAGAF,EAAAD,QAvBA,GAAAD,KAqCA,OATAF,GAAAQ,EAAAT,EAGAC,EAAAS,EAAAP,EAGAF,EAAAU,EAAA,GAGAV,EAAA,KDMM,SAASI,EAAQD,EAASH,GE3ChCA,EAAA,GAEAA,EAAA,GAEAW,OFmDM,SAASP,EAAQD,EAASH,GGxDhCI,EAAAD,QAAAH,EAAAU,EAAA,cH8DM,SAASN,EAAQD,GI9DvB,QAAAQ,KACAC,QAAAC,IAAAC,QAGAH\",\"file\":\"bundle.js\",\"sourcesContent\":[\"/******/ (function(modules) { // webpackBootstrap\\n/******/ \\t// The module cache\\n/******/ \\tvar installedModules = {};\\n/******/\\n/******/ \\t// The require function\\n/******/ \\tfunction __webpack_require__(moduleId) {\\n/******/\\n/******/ \\t\\t// Check if module is in cache\\n/******/ \\t\\tif(installedModules[moduleId])\\n/******/ \\t\\t\\treturn installedModules[moduleId].exports;\\n/******/\\n/******/ \\t\\t// Create a new module (and put it into the cache)\\n/******/ \\t\\tvar module = installedModules[moduleId] = {\\n/******/ \\t\\t\\texports: {},\\n/******/ \\t\\t\\tid: moduleId,\\n/******/ \\t\\t\\tloaded: false\\n/******/ \\t\\t};\\n/******/\\n/******/ \\t\\t// Execute the module function\\n/******/ \\t\\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\\n/******/\\n/******/ \\t\\t// Flag the module as loaded\\n/******/ \\t\\tmodule.loaded = true;\\n/******/\\n/******/ \\t\\t// Return the exports of the module\\n/******/ \\t\\treturn module.exports;\\n/******/ \\t}\\n/******/\\n/******/\\n/******/ \\t// expose the modules object (__webpack_modules__)\\n/******/ \\t__webpack_require__.m = modules;\\n/******/\\n/******/ \\t// expose the module cache\\n/******/ \\t__webpack_require__.c = installedModules;\\n/******/\\n/******/ \\t// __webpack_public_path__\\n/******/ \\t__webpack_require__.p = \\\"\\\";\\n/******/\\n/******/ \\t// Load entry module and return exports\\n/******/ \\treturn __webpack_require__(0);\\n/******/ })\\n/************************************************************************/\\n/******/ ([\\n/* 0 */\\n/***/ function(module, exports, __webpack_require__) {\\n\\n\\t// Webpack\\n\\t__webpack_require__(1)\\n\\t\\n\\t__webpack_require__(2)\\n\\t\\n\\tfoo()\\n\\n\\n/***/ },\\n/* 1 */\\n/***/ function(module, exports, __webpack_require__) {\\n\\n\\tmodule.exports = __webpack_require__.p + \\\"index.html\\\"\\n\\n/***/ },\\n/* 2 */\\n/***/ function(module, exports) {\\n\\n\\tfunction foo() {\\n\\t    console.log(foobar)\\n\\t}\\n\\t\\n\\tfoo()\\n\\n\\n/***/ }\\n/******/ ]);\\n\\n\\n/** WEBPACK FOOTER **\\n ** bundle.js\\n **/\",\" \\t// The module cache\\n \\tvar installedModules = {};\\n\\n \\t// The require function\\n \\tfunction __webpack_require__(moduleId) {\\n\\n \\t\\t// Check if module is in cache\\n \\t\\tif(installedModules[moduleId])\\n \\t\\t\\treturn installedModules[moduleId].exports;\\n\\n \\t\\t// Create a new module (and put it into the cache)\\n \\t\\tvar module = installedModules[moduleId] = {\\n \\t\\t\\texports: {},\\n \\t\\t\\tid: moduleId,\\n \\t\\t\\tloaded: false\\n \\t\\t};\\n\\n \\t\\t// Execute the module function\\n \\t\\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\\n\\n \\t\\t// Flag the module as loaded\\n \\t\\tmodule.loaded = true;\\n\\n \\t\\t// Return the exports of the module\\n \\t\\treturn module.exports;\\n \\t}\\n\\n\\n \\t// expose the modules object (__webpack_modules__)\\n \\t__webpack_require__.m = modules;\\n\\n \\t// expose the module cache\\n \\t__webpack_require__.c = installedModules;\\n\\n \\t// __webpack_public_path__\\n \\t__webpack_require__.p = \\\"\\\";\\n\\n \\t// Load entry module and return exports\\n \\treturn __webpack_require__(0);\\n\\n\\n\\n/** WEBPACK FOOTER **\\n ** webpack/bootstrap 6002740481c9666b0d38\\n **/\",\"// Webpack\\nrequire('../index.html')\\n\\nrequire('./app')\\n\\nfoo()\\n\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./scripts/index.js\\n ** module id = 0\\n ** module chunks = 0\\n **/\",\"module.exports = __webpack_public_path__ + \\\"index.html\\\"\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./index.html\\n ** module id = 1\\n ** module chunks = 0\\n **/\",\"function foo() {\\n    console.log(foobar)\\n}\\n\\nfoo()\\n\\n\\n\\n/*****************\\n ** WEBPACK FOOTER\\n ** ./scripts/app.js\\n ** module id = 2\\n ** module chunks = 0\\n **/\"],\"sourceRoot\":\"\"}}"
