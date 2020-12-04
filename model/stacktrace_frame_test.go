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

package model

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/tests"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/sourcemap/test"
	"github.com/elastic/apm-server/transform"
)

func TestStacktraceFrameTransform(t *testing.T) {
	filename, classname := "some file", "foo"
	lineno := 1
	colno := 55
	path := "~/./some/abs_path"
	context := "context"
	fct := "some function"
	module := "some_module"
	libraryFrame := true
	tests := []struct {
		StFrame StacktraceFrame
		Output  common.MapStr
		Msg     string
	}{
		{
			StFrame: StacktraceFrame{Filename: &filename, Lineno: &lineno},
			Output: common.MapStr{
				"filename":              filename,
				"line":                  common.MapStr{"number": lineno},
				"exclude_from_grouping": false,
			},
			Msg: "Minimal StacktraceFrame",
		},
		{
			StFrame: StacktraceFrame{
				AbsPath:      &path,
				Filename:     &filename,
				Classname:    &classname,
				Lineno:       &lineno,
				Colno:        &colno,
				ContextLine:  &context,
				Module:       &module,
				Function:     &fct,
				LibraryFrame: &libraryFrame,
				Vars:         map[string]interface{}{"k1": "v1", "k2": "v2"},
				PreContext:   []string{"prec1", "prec2"},
				PostContext:  []string{"postc1", "postc2"},
			},
			Output: common.MapStr{
				"abs_path":      "~/./some/abs_path",
				"filename":      "some file",
				"classname":     "foo",
				"function":      "some function",
				"module":        "some_module",
				"library_frame": true,
				"vars":          common.MapStr{"k1": "v1", "k2": "v2"},
				"context": common.MapStr{
					"pre":  []string{"prec1", "prec2"},
					"post": []string{"postc1", "postc2"},
				},
				"line": common.MapStr{
					"number":  1,
					"column":  55,
					"context": "context",
				},
				"exclude_from_grouping": false,
			},
			Msg: "Full StacktraceFrame",
		},
	}

	for idx, test := range tests {
		output := test.StFrame.transform(&transform.Config{}, true)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestSourcemap_Apply(t *testing.T) {

	name, version, col, line, path := "myservice", "2.1.4", 10, 15, "/../a/path"
	validService := func() *Service {
		return &Service{Name: name, Version: version}
	}
	validFrame := func() *StacktraceFrame {
		return &StacktraceFrame{Colno: &col, Lineno: &line, AbsPath: &path}
	}

	t.Run("frame", func(t *testing.T) {
		for name, tc := range map[string]struct {
			frame *StacktraceFrame

			expectedErrorMsg string
		}{
			"noColumn": {
				frame:            &StacktraceFrame{},
				expectedErrorMsg: "Colno mandatory"},
			"noLine": {
				frame:            &StacktraceFrame{Colno: &col},
				expectedErrorMsg: "Lineno mandatory"},
			"noPath": {
				frame:            &StacktraceFrame{Colno: &col, Lineno: &line},
				expectedErrorMsg: "AbsPath mandatory",
			},
		} {
			t.Run(name, func(t *testing.T) {
				function, msg := tc.frame.applySourcemap(context.Background(), &sourcemap.Store{}, validService(), "foo")
				assert.Equal(t, "foo", function)
				assert.Contains(t, msg, tc.expectedErrorMsg)
				assert.Equal(t, new(bool), tc.frame.SourcemapUpdated)
				require.NotNil(t, tc.frame.SourcemapError)
				assert.Contains(t, *tc.frame.SourcemapError, msg)
				assert.Zero(t, tc.frame.Original)
			})
		}
	})

	t.Run("errorPerFrame", func(t *testing.T) {
		for name, tc := range map[string]struct {
			store            *sourcemap.Store
			expectedErrorMsg string
		}{
			"noSourcemap": {store: testSourcemapStore(t, test.ESClientWithSourcemapNotFound(t)),
				expectedErrorMsg: "No Sourcemap available"},
			"noMapping": {store: testSourcemapStore(t, test.ESClientWithValidSourcemap(t)),
				expectedErrorMsg: "No Sourcemap found for Lineno",
			},
		} {
			t.Run(name, func(t *testing.T) {
				frame := validFrame()
				function, msg := frame.applySourcemap(context.Background(), tc.store, validService(), "xyz")
				assert.Equal(t, "xyz", function)
				require.Contains(t, msg, tc.expectedErrorMsg)
				assert.NotZero(t, frame.SourcemapError)
				assert.Equal(t, new(bool), frame.SourcemapUpdated)
			})
		}
	})

	t.Run("mappingError", func(t *testing.T) {
		for name, tc := range map[string]struct {
			store            *sourcemap.Store
			expectedErrorMsg string
		}{
			"ESUnavailable": {store: testSourcemapStore(t, test.ESClientUnavailable(t)),
				expectedErrorMsg: "client error"},
			"invalidSourcemap": {store: testSourcemapStore(t, test.ESClientWithInvalidSourcemap(t)),
				expectedErrorMsg: "Could not parse Sourcemap."},
			"unsupportedSourcemap": {store: testSourcemapStore(t, test.ESClientWithUnsupportedSourcemap(t)),
				expectedErrorMsg: "only 3rd version is supported"},
		} {
			t.Run(name, func(t *testing.T) {
				frame := validFrame()
				function, msg := frame.applySourcemap(context.Background(), tc.store, validService(), "xyz")
				assert.Equal(t, "xyz", function)
				require.Contains(t, msg, tc.expectedErrorMsg)
				assert.NotZero(t, msg)
				assert.Zero(t, frame.SourcemapUpdated)
				assert.Zero(t, frame.SourcemapError)
			})
		}
	})

	t.Run("mapping", func(t *testing.T) {

		for name, tc := range map[string]struct {
			origCol, origLine int
			origPath          string

			function, file, path, ctxLine string
			preCtx, postCtx               []string
			col, line                     int
		}{
			"withFunction": {origCol: 67, origLine: 1, origPath: "/../a/path",
				function: "exports", file: "", path: "/a/path", ctxLine: " \t\t\texports: {},", col: 0, line: 13,
				preCtx:  []string{" \t\tif(installedModules[moduleId])", " \t\t\treturn installedModules[moduleId].exports;", "", " \t\t// Create a new module (and put it into the cache)", " \t\tvar module = installedModules[moduleId] = {"},
				postCtx: []string{" \t\t\tid: moduleId,", " \t\t\tloaded: false", " \t\t};", "", " \t\t// Execute the module function"}},
			"withFilename": {origCol: 7, origLine: 1, origPath: "/../a/path",
				function: "<unknown>", file: "webpack:///bundle.js", path: "/a/path",
				ctxLine: "/******/ (function(modules) { // webpackBootstrap",
				preCtx:  []string(nil),
				postCtx: []string{"/******/ \t// The module cache", "/******/ \tvar installedModules = {};", "/******/", "/******/ \t// The require function", "/******/ \tfunction __webpack_require__(moduleId) {"},
				col:     9, line: 1},
			"withoutFilename": {origCol: 23, origLine: 1, origPath: "/../a/path",
				function: "__webpack_require__", file: "", path: "/a/path", ctxLine: " \tfunction __webpack_require__(moduleId) {",
				preCtx:  []string{" \t// The module cache", " \tvar installedModules = {};", "", " \t// The require function"},
				postCtx: []string{"", " \t\t// Check if module is in cache", " \t\tif(installedModules[moduleId])", " \t\t\treturn installedModules[moduleId].exports;", ""},
				col:     0, line: 5},
		} {
			t.Run(name, func(t *testing.T) {
				frame := &StacktraceFrame{Colno: &tc.origCol, Lineno: &tc.origLine, AbsPath: &tc.origPath}

				prevFunction := "xyz"
				function, msg := frame.applySourcemap(context.Background(), testSourcemapStore(t, test.ESClientWithValidSourcemap(t)), validService(), prevFunction)
				require.Empty(t, msg)
				assert.Zero(t, frame.SourcemapError)
				updated := true
				assert.Equal(t, &updated, frame.SourcemapUpdated)

				assert.Equal(t, tc.function, function)
				assert.Equal(t, prevFunction, *frame.Function)
				assert.Equal(t, tc.col, *frame.Colno)
				assert.Equal(t, tc.line, *frame.Lineno)
				assert.Equal(t, tc.path, *frame.AbsPath)
				assert.Equal(t, tc.ctxLine, *frame.ContextLine)
				assert.Equal(t, tc.preCtx, frame.PreContext)
				assert.Equal(t, tc.postCtx, frame.PostContext)
				if tc.file == "" {
					assert.Nil(t, frame.Filename)
				} else {
					assert.Equal(t, tc.file, *frame.Filename)
				}
				assert.NotZero(t, frame.Original)
			})
		}
	})
}

func TestIsLibraryFrame(t *testing.T) {
	assert.False(t, (&StacktraceFrame{}).IsLibraryFrame())
	assert.False(t, (&StacktraceFrame{LibraryFrame: new(bool)}).IsLibraryFrame())
	libFrame := true
	assert.True(t, (&StacktraceFrame{LibraryFrame: &libFrame}).IsLibraryFrame())
}

func TestIsSourcemapApplied(t *testing.T) {
	assert.False(t, (&StacktraceFrame{}).IsSourcemapApplied())

	fr := StacktraceFrame{SourcemapUpdated: new(bool)}
	assert.False(t, fr.IsSourcemapApplied())

	libFrame := true
	fr = StacktraceFrame{SourcemapUpdated: &libFrame}
	assert.True(t, fr.IsSourcemapApplied())
}

func TestExcludeFromGroupingKey(t *testing.T) {
	tests := []struct {
		fr      StacktraceFrame
		pattern string
		exclude bool
	}{
		{
			fr:      StacktraceFrame{},
			pattern: "",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/webpack")},
			pattern: "",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/webpack")},
			pattern: "/webpack/tmp",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("")},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/webpack")},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/webpack/test/e2e/general-usecase/app.e2e-bundle.js")},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/filename")},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("/filename/a")},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: tests.StringPtr("webpack")},
			pattern: "^/webpack",
			exclude: false,
		},
	}

	for idx, test := range tests {
		var excludePattern *regexp.Regexp
		if test.pattern != "" {
			excludePattern = regexp.MustCompile(test.pattern)
		}

		out := test.fr.transform(&transform.Config{
			RUM: transform.RUMConfig{ExcludeFromGrouping: excludePattern},
		}, true)
		exclude := out["exclude_from_grouping"]
		assert.Equal(t, test.exclude, exclude,
			fmt.Sprintf("(%v): Pattern: %v, Filename: %v, expected to be excluded: %v", idx, test.pattern, test.fr.Filename, test.exclude))
	}
}

func TestLibraryFrame(t *testing.T) {

	truthy := true
	falsy := false
	path := "/~/a/b"
	tests := []struct {
		fr               StacktraceFrame
		libraryPattern   *regexp.Regexp
		libraryFrame     *bool
		origLibraryFrame *bool
		msg              string
	}{
		{fr: StacktraceFrame{},
			libraryFrame:     nil,
			origLibraryFrame: nil,
			msg:              "Empty StacktraceFrame, empty config"},
		{fr: StacktraceFrame{AbsPath: &path},
			libraryFrame:     nil,
			origLibraryFrame: nil,
			msg:              "No pattern"},
		{fr: StacktraceFrame{AbsPath: &path},
			libraryPattern:   regexp.MustCompile(""),
			libraryFrame:     &truthy,
			origLibraryFrame: nil,
			msg:              "Empty pattern"},
		{fr: StacktraceFrame{LibraryFrame: &falsy},
			libraryPattern:   regexp.MustCompile("~"),
			libraryFrame:     &falsy,
			origLibraryFrame: &falsy,
			msg:              "Empty StacktraceFrame"},
		{fr: StacktraceFrame{AbsPath: &path, LibraryFrame: &truthy},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     &falsy,
			origLibraryFrame: &truthy,
			msg:              "AbsPath given, no Match"},
		{fr: StacktraceFrame{Filename: tests.StringPtr("myFile.js"), LibraryFrame: &truthy},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     &falsy,
			origLibraryFrame: &truthy,
			msg:              "Filename given, no Match"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: tests.StringPtr("myFile.js")},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     &falsy,
			origLibraryFrame: nil,
			msg:              "AbsPath and Filename given, no Match"},
		{fr: StacktraceFrame{Filename: tests.StringPtr("/tmp")},
			libraryPattern:   regexp.MustCompile("/tmp"),
			libraryFrame:     &truthy,
			origLibraryFrame: nil,
			msg:              "Filename matching"},
		{fr: StacktraceFrame{AbsPath: &path, LibraryFrame: &falsy},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     &truthy,
			origLibraryFrame: &falsy,
			msg:              "AbsPath matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: tests.StringPtr("/a/b/c")},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     &truthy,
			origLibraryFrame: nil,
			msg:              "AbsPath matching, Filename not matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: tests.StringPtr("/a/b/c")},
			libraryPattern:   regexp.MustCompile("/a/b/c"),
			libraryFrame:     &truthy,
			origLibraryFrame: nil,
			msg:              "AbsPath not matching, Filename matching"},
		{fr: StacktraceFrame{AbsPath: &path, Filename: tests.StringPtr("~/a/b/c")},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     &truthy,
			origLibraryFrame: nil,
			msg:              "AbsPath and Filename matching"},
	}

	for _, test := range tests {
		cfg := transform.Config{
			RUM: transform.RUMConfig{
				LibraryPattern: test.libraryPattern,
			},
		}
		out := test.fr.transform(&cfg, true)["library_frame"]
		libFrame := test.fr.LibraryFrame
		origLibFrame := test.fr.Original.LibraryFrame
		if test.libraryFrame == nil {
			assert.Nil(t, out, test.msg)
			assert.Nil(t, libFrame, test.msg)
		} else {
			assert.Equal(t, *test.libraryFrame, out, test.msg)
			assert.Equal(t, *test.libraryFrame, *libFrame, test.msg)
		}
		if test.origLibraryFrame == nil {
			assert.Nil(t, origLibFrame, test.msg)
		} else {
			assert.Equal(t, *test.origLibraryFrame, *origLibFrame, test.msg)
		}
	}
}

func testSourcemapStore(t *testing.T, client elasticsearch.Client) *sourcemap.Store {
	store, err := sourcemap.NewStore(client, "apm-*sourcemap*", time.Minute)
	require.NoError(t, err)
	return store
}
