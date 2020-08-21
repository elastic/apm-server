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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/sourcemap/test"
	"github.com/elastic/apm-server/transform"
)

func TestStacktraceTransform(t *testing.T) {
	colno := 1
	l4, l5, l6, l8 := 4, 5, 6, 8
	fct := "original function"
	origFilename, webpackFilename := "original filename", "/webpack"
	absPath, serviceName := "original path", "service1"
	service := Service{Name: serviceName}

	tests := []struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		{
			Stacktrace: Stacktrace{},
			Output:     nil,
			Msg:        "Empty Stacktrace",
		},
		{
			Stacktrace: Stacktrace{&StacktraceFrame{}},
			Output:     []common.MapStr{{"exclude_from_grouping": false}},
			Msg:        "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &origFilename,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: &l6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: &l8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l5,
					Filename: &origFilename,
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: &webpackFilename,
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": "original path", "filename": "original filename", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 4},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 8},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "original filename", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 5},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "/webpack",
					"line":                  common.MapStr{"column": 1, "number": 4},
					"exclude_from_grouping": false,
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.transform(context.Background(), &transform.Config{}, false, &service)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestStacktraceTransformWithSourcemapping(t *testing.T) {
	int1, int6, int7, int67 := 1, 6, 7, 67
	fct1, fct2 := "function foo", "function bar"
	absPath, serviceName, serviceVersion := "/../a/c", "service1", "2.4.1"
	origFilename, appliedFilename := "original filename", "myfilename"
	service := Service{Name: serviceName, Version: serviceVersion}

	for name, tc := range map[string]struct {
		Stacktrace Stacktrace
		Output     []common.MapStr
		Msg        string
	}{
		"emptyStacktrace": {
			Stacktrace: Stacktrace{},
			Output:     nil,
		},
		"emptyFrame": {
			Stacktrace: Stacktrace{&StacktraceFrame{}},
			Output: []common.MapStr{
				{"exclude_from_grouping": false,
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				},
			},
		},
		"noLineno": {
			Stacktrace: Stacktrace{&StacktraceFrame{Colno: &int1}},
			Output: []common.MapStr{
				{"line": common.MapStr{"column": 1},
					"exclude_from_grouping": false,
					"sourcemap": common.MapStr{
						"error":   "Lineno mandatory for sourcemapping.",
						"updated": false,
					},
				},
			},
		},
		"sourcemapApplied": {
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &int7,
					Lineno:   &int1,
					Filename: &origFilename,
					Function: &fct1,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &int67,
					Lineno:   &int1,
					Filename: &appliedFilename,
					Function: &fct2,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &int7,
					Lineno:   &int1,
					Filename: &appliedFilename,
					Function: &fct2,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &int1, Lineno: &int6, Function: &fct2, AbsPath: &absPath},
			},
			Output: []common.MapStr{
				{
					"abs_path": "/a/c",
					"filename": "webpack:///bundle.js",
					"function": "exports",
					"context": common.MapStr{
						"post": []string{"/******/ \t// The module cache", "/******/ \tvar installedModules = {};", "/******/", "/******/ \t// The require function", "/******/ \tfunction __webpack_require__(moduleId) {"}},
					"line": common.MapStr{
						"column":  9,
						"number":  1,
						"context": "/******/ (function(modules) { // webpackBootstrap"},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "/../a/c",
						"colno":    7,
						"filename": "original filename",
						"function": "function foo",
						"lineno":   1,
					},
				},
				{
					"abs_path": "/a/c",
					"filename": "myfilename",
					"function": "<unknown>", //prev function
					"context": common.MapStr{
						"post": []string{" \t\t\tid: moduleId,", " \t\t\tloaded: false", " \t\t};", "", " \t\t// Execute the module function"},
						"pre":  []string{" \t\tif(installedModules[moduleId])", " \t\t\treturn installedModules[moduleId].exports;", "", " \t\t// Create a new module (and put it into the cache)", " \t\tvar module = installedModules[moduleId] = {"}},
					"line": common.MapStr{
						"column":  0,
						"number":  13,
						"context": " \t\t\texports: {},"},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "/../a/c",
						"colno":    67,
						"filename": "myfilename",
						"function": "function bar",
						"lineno":   1,
					},
				},
				{
					"abs_path": "/a/c",
					"filename": "webpack:///bundle.js",
					"function": "<anonymous>", //prev function
					"context": common.MapStr{
						"post": []string{"/******/ \t// The module cache", "/******/ \tvar installedModules = {};", "/******/", "/******/ \t// The require function", "/******/ \tfunction __webpack_require__(moduleId) {"}},
					"line": common.MapStr{
						"column":  9,
						"number":  1,
						"context": "/******/ (function(modules) { // webpackBootstrap"},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "/../a/c",
						"colno":    7,
						"filename": "myfilename",
						"function": "function bar",
						"lineno":   1,
					},
				},
				{
					"abs_path":              "/../a/c",
					"function":              fct2,
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": false, "error": "No Sourcemap found for Lineno 6, Colno 1"},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &transform.Config{
				RUM: transform.RUMConfig{
					SourcemapStore: testSourcemapStore(t, test.ESClientWithValidSourcemap(t)),
				},
			}

			// run `Stacktrace.transform` twice to ensure method is idempotent
			tc.Stacktrace.transform(context.Background(), cfg, true, &service)
			output := tc.Stacktrace.transform(context.Background(), cfg, true, &service)
			assert.Equal(t, tc.Output, output)
		})
	}
}
