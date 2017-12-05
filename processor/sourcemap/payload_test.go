package sourcemap

import (
	"testing"

	"encoding/json"

	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/libbeat/common"
)

func getStr(data common.MapStr, key string) string {
	rs, _ := data.GetValue(key)
	return rs.(string)
}

func getFloat(data common.MapStr, key string) float64 {
	rs, _ := data.GetValue(key)
	return rs.(float64)
}

func getStrSlice(data common.MapStr, key string) []string {
	l, _ := data.GetValue(key)
	var rs []string
	for _, i := range l.([]interface{}) {
		rs = append(rs, i.(string))
	}
	return rs
}

func TestPayloadTransform(t *testing.T) {
	var payload payload
	fileBytes, err := tests.LoadValidData("sourcemap")
	assert.NoError(t, err)
	json.Unmarshal(fileBytes, &payload)

	rs := payload.transform()

	assert.Len(t, rs, 1)
	event := rs[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)

	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "js/bundle.js", getStr(output, "bundle_filepath"))
	assert.Equal(t, "service", getStr(output, "service.name"))
	assert.Equal(t, "1", getStr(output, "service.version"))
	assert.Equal(t, float64(3), getFloat(output, "sourcemap.version"))
	assert.Equal(t,
		[]string{"webpack:///bundle.js", "webpack:///webpack/bootstrap 6002740481c9666b0d38", "webpack:///./scripts/index.js", "webpack:///./index.html", "webpack:///./scripts/app.js"},
		getStrSlice(output, "sourcemap.sources"))
	assert.Equal(t,
		[]string{"modules", "__webpack_require__", "moduleId", "installedModules", "exports", "module", "id", "loaded", "call", "m", "c", "p", "foo", "console", "log", "foobar"},
		getStrSlice(output, "sourcemap.names"))
	assert.Equal(t,
		"CAAS,SAAUA,GCInB,QAAAC,GAAAC,GAGA,GAAAC,EAAAD,GACA,MAAAC,GAAAD,GAAAE,OAGA,IAAAC,GAAAF,EAAAD,IACAE,WACAE,GAAAJ,EACAK,QAAA,EAUA,OANAP,GAAAE,GAAAM,KAAAH,EAAAD,QAAAC,IAAAD,QAAAH,GAGAI,EAAAE,QAAA,EAGAF,EAAAD,QAvBA,GAAAD,KAqCA,OATAF,GAAAQ,EAAAT,EAGAC,EAAAS,EAAAP,EAGAF,EAAAU,EAAA,GAGAV,EAAA,KDMM,SAASI,EAAQD,EAASH,GE3ChCA,EAAA,GAEAA,EAAA,GAEAW,OFmDM,SAASP,EAAQD,EAASH,GGxDhCI,EAAAD,QAAAH,EAAAU,EAAA,cH8DM,SAASN,EAAQD,GI9DvB,QAAAQ,KACAC,QAAAC,IAAAC,QAGAH",
		getStr(output, "sourcemap.mappings"))
	assert.Equal(t, "bundle.js", getStr(output, "sourcemap.file"))
	assert.Equal(t,
		[]string{"/******/ (function(modules) { // webpackBootstrap\n/******/ \t// The module cache\n/******/ \tvar installedModules = {};\n/******/\n/******/ \t// The require function\n/******/ \tfunction __webpack_require__(moduleId) {\n/******/\n/******/ \t\t// Check if module is in cache\n/******/ \t\tif(installedModules[moduleId])\n/******/ \t\t\treturn installedModules[moduleId].exports;\n/******/\n/******/ \t\t// Create a new module (and put it into the cache)\n/******/ \t\tvar module = installedModules[moduleId] = {\n/******/ \t\t\texports: {},\n/******/ \t\t\tid: moduleId,\n/******/ \t\t\tloaded: false\n/******/ \t\t};\n/******/\n/******/ \t\t// Execute the module function\n/******/ \t\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\n/******/\n/******/ \t\t// Flag the module as loaded\n/******/ \t\tmodule.loaded = true;\n/******/\n/******/ \t\t// Return the exports of the module\n/******/ \t\treturn module.exports;\n/******/ \t}\n/******/\n/******/\n/******/ \t// expose the modules object (__webpack_modules__)\n/******/ \t__webpack_require__.m = modules;\n/******/\n/******/ \t// expose the module cache\n/******/ \t__webpack_require__.c = installedModules;\n/******/\n/******/ \t// __webpack_public_path__\n/******/ \t__webpack_require__.p = \"\";\n/******/\n/******/ \t// Load entry module and return exports\n/******/ \treturn __webpack_require__(0);\n/******/ })\n/************************************************************************/\n/******/ ([\n/* 0 */\n/***/ function(module, exports, __webpack_require__) {\n\n\t// Webpack\n\t__webpack_require__(1)\n\t\n\t__webpack_require__(2)\n\t\n\tfoo()\n\n\n/***/ },\n/* 1 */\n/***/ function(module, exports, __webpack_require__) {\n\n\tmodule.exports = __webpack_require__.p + \"index.html\"\n\n/***/ },\n/* 2 */\n/***/ function(module, exports) {\n\n\tfunction foo() {\n\t    console.log(foobar)\n\t}\n\t\n\tfoo()\n\n\n/***/ }\n/******/ ]);\n\n\n/** WEBPACK FOOTER **\n ** bundle.js\n **/",
			" \t// The module cache\n \tvar installedModules = {};\n\n \t// The require function\n \tfunction __webpack_require__(moduleId) {\n\n \t\t// Check if module is in cache\n \t\tif(installedModules[moduleId])\n \t\t\treturn installedModules[moduleId].exports;\n\n \t\t// Create a new module (and put it into the cache)\n \t\tvar module = installedModules[moduleId] = {\n \t\t\texports: {},\n \t\t\tid: moduleId,\n \t\t\tloaded: false\n \t\t};\n\n \t\t// Execute the module function\n \t\tmodules[moduleId].call(module.exports, module, module.exports, __webpack_require__);\n\n \t\t// Flag the module as loaded\n \t\tmodule.loaded = true;\n\n \t\t// Return the exports of the module\n \t\treturn module.exports;\n \t}\n\n\n \t// expose the modules object (__webpack_modules__)\n \t__webpack_require__.m = modules;\n\n \t// expose the module cache\n \t__webpack_require__.c = installedModules;\n\n \t// __webpack_public_path__\n \t__webpack_require__.p = \"\";\n\n \t// Load entry module and return exports\n \treturn __webpack_require__(0);\n\n\n\n/** WEBPACK FOOTER **\n ** webpack/bootstrap 6002740481c9666b0d38\n **/",
			"// Webpack\nrequire('../index.html')\n\nrequire('./app')\n\nfoo()\n\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./scripts/index.js\n ** module id = 0\n ** module chunks = 0\n **/",
			"module.exports = __webpack_public_path__ + \"index.html\"\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./index.html\n ** module id = 1\n ** module chunks = 0\n **/",
			"function foo() {\n    console.log(foobar)\n}\n\nfoo()\n\n\n\n/*****************\n ** WEBPACK FOOTER\n ** ./scripts/app.js\n ** module id = 2\n ** module chunks = 0\n **/"},
		getStrSlice(output, "sourcemap.sourcesContent"))
	assert.Equal(t, "", getStr(output, "sourcemap.sourceRoot"))
}

func TestParseSourcemaps(t *testing.T) {
	fileBytes, err := tests.LoadData("tests/data/valid/sourcemap/bundle.min.map")
	assert.NoError(t, err)
	parser, err := s.Parse("", fileBytes)
	assert.NoError(t, err)

	source, _, _, _, ok := parser.Source(1, 9)
	assert.True(t, ok)
	assert.Equal(t, "webpack:///bundle.js", source)
}
