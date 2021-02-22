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

package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"

	"github.com/elastic/apm-server/elasticsearch/estest"
)

//ValidSourcemap represents an example for a valid sourcemap string
var ValidSourcemap = `{
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

// ESClientWithValidSourcemap returns an elasticsearch client that will always return a document containing
// a valid sourcemap.
func ESClientWithValidSourcemap(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusOK, validSourcemapFromES()))
	require.NoError(t, err)
	return client
}

// ESClientUnavailable returns an elasticsearch client that will always return a client error, mimicking an
// unavailable Elasticsearch server.
func ESClientUnavailable(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, -1, nil))
	require.NoError(t, err)
	return client
}

// ESClientWithInvalidSourcemap returns an elasticsearch client that will always return a document containing
// an invalid sourcemap.
func ESClientWithInvalidSourcemap(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusOK, invalidSourcemapFromES()))
	require.NoError(t, err)
	return client
}

// ESClientWithUnsupportedSourcemap returns an elasticsearch client that will always return a document containing
// a sourcemap with an unsupported version.
func ESClientWithUnsupportedSourcemap(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusOK, sourcemapUnsupportedVersionFromES()))
	require.NoError(t, err)
	return client
}

// ESClientWithSourcemapNotFound returns an elasticsearch client that will always return a not found error
func ESClientWithSourcemapNotFound(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusNotFound, sourcemapNotFoundFromES()))
	require.NoError(t, err)
	return client
}

// ESClientWithSourcemapIndicatedNotFound returns an elasticsearch client that will always return a result indicating
// a sourcemap exists but it actually doesn't contain it. This is an edge case that usually shouldn't happen.
func ESClientWithSourcemapIndicatedNotFound(t *testing.T) elasticsearch.Client {
	client, err := estest.NewElasticsearchClient(estest.NewTransport(t, http.StatusOK, sourcemapIndicatedNotFoundFromES()))
	require.NoError(t, err)
	return client
}

func validSourcemapFromES() map[string]interface{} {
	return map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": 1},
			"hits": []map[string]interface{}{
				{"_source": map[string]interface{}{
					"sourcemap": map[string]interface{}{
						"sourcemap": ValidSourcemap}}}}}}
}

func sourcemapNotFoundFromES() map[string]interface{} {
	return map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": 0}}}
}

func sourcemapIndicatedNotFoundFromES() map[string]interface{} {
	return map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": 1},
			"hits":  []map[string]interface{}{}}}
}

func invalidSourcemapFromES() map[string]interface{} {
	return map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": 1},
			"hits": []map[string]interface{}{
				{"_source": map[string]interface{}{
					"sourcemap": map[string]interface{}{
						"sourcemap": "foo"}}}}}}
}

func sourcemapUnsupportedVersionFromES() map[string]interface{} {
	return map[string]interface{}{
		"hits": map[string]interface{}{
			"total": map[string]interface{}{"value": 1},
			"hits": []map[string]interface{}{
				{"_source": map[string]interface{}{
					"sourcemap": map[string]interface{}{
						"sourcemap": `{
		   "version": 1,
		   "sources": ["webpack:///bundle.js"],
		   "names": [],
		   "mappings": "CAAS",
		   "file": "bundle.js",
		   "sourcesContent": [],
		   "sourceRoot": ""
		}`}}}}}}
}
