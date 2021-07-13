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

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/transform"
)

func TestStacktraceTransform(t *testing.T) {
	originalLineno := 111
	originalColno := 222
	originalFunction := "original function"
	originalFilename := "original filename"
	originalModule := "original module"
	originalClassname := "original classname"
	originalAbsPath := "original path"

	mappedLineno := 333
	mappedColno := 444
	mappedFunction := "mapped function"
	mappedFilename := "mapped filename"
	mappedClassname := "mapped classname"
	mappedAbsPath := "mapped path"

	vars := common.MapStr{"a": "abc", "b": 123}

	contextLine := "context line"
	preContext := []string{"before1", "before2"}
	postContext := []string{"after1", "after2"}

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
			Stacktrace: Stacktrace{{
				Colno:        &originalColno,
				Lineno:       &originalLineno,
				Filename:     originalFilename,
				Function:     originalFunction,
				Classname:    originalClassname,
				Module:       originalModule,
				AbsPath:      originalAbsPath,
				LibraryFrame: newBool(true),
				Vars:         vars,
			}},
			Output: []common.MapStr{{
				"abs_path":  "original path",
				"filename":  "original filename",
				"function":  "original function",
				"classname": "original classname",
				"module":    "original module",
				"line": common.MapStr{
					"number": 111,
					"column": 222,
				},
				"exclude_from_grouping": false,
				"library_frame":         true,
				"vars":                  vars,
			}},
			Msg: "unmapped stacktrace",
		},
		{
			Stacktrace: Stacktrace{{
				Colno:     &mappedColno,
				Lineno:    &mappedLineno,
				Filename:  mappedFilename,
				Function:  mappedFunction,
				Classname: mappedClassname,
				AbsPath:   mappedAbsPath,
				Original: Original{
					Colno:     &originalColno,
					Lineno:    &originalLineno,
					Filename:  originalFilename,
					Function:  originalFunction,
					Classname: originalClassname,
					AbsPath:   originalAbsPath,
				},
				ExcludeFromGrouping: true,
				SourcemapUpdated:    true,
				SourcemapError:      "boom",
				ContextLine:         contextLine,
				PreContext:          preContext,
				PostContext:         postContext,
			}},
			Output: []common.MapStr{{
				"abs_path":  "mapped path",
				"filename":  "mapped filename",
				"function":  "mapped function",
				"classname": "mapped classname",
				"line": common.MapStr{
					"number":  333,
					"column":  444,
					"context": "context line",
				},
				"context": common.MapStr{
					"pre":  preContext,
					"post": postContext,
				},
				"original": common.MapStr{
					"abs_path":  "original path",
					"filename":  "original filename",
					"function":  "original function",
					"classname": "original classname",
					"lineno":    111,
					"colno":     222,
				},
				"exclude_from_grouping": true,
				"sourcemap": common.MapStr{
					"updated": true,
					"error":   "boom",
				},
			}},
			Msg: "mapped stacktrace",
		},
	}

	for idx, test := range tests {
		output := test.Stacktrace.transform(context.Background(), &transform.Config{}, false)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestStacktraceFrameIsLibraryFrame(t *testing.T) {
	assert.False(t, (&StacktraceFrame{}).IsLibraryFrame())
	assert.False(t, (&StacktraceFrame{LibraryFrame: new(bool)}).IsLibraryFrame())
	libFrame := true
	assert.True(t, (&StacktraceFrame{LibraryFrame: &libFrame}).IsLibraryFrame())
}

func TestStacktraceFrameExcludeFromGroupingKey(t *testing.T) {
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
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "/webpack/tmp",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: ""},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack"},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: "/webpack/test/e2e/general-usecase/app.e2e-bundle.js"},
			pattern: "^/webpack",
			exclude: true,
		},
		{
			fr:      StacktraceFrame{Filename: "/filename"},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "/filename/a"},
			pattern: "^/webpack",
			exclude: false,
		},
		{
			fr:      StacktraceFrame{Filename: "webpack"},
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
		message := fmt.Sprintf(
			"(%v): Pattern: %v, Filename: %v, expected to be excluded: %v",
			idx, test.pattern, test.fr.Filename, test.exclude,
		)
		assert.Equal(t, test.exclude, exclude, message)
	}
}

func TestStacktraceFrameLibraryFrame(t *testing.T) {
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
		{fr: StacktraceFrame{AbsPath: path},
			libraryFrame:     nil,
			origLibraryFrame: nil,
			msg:              "No pattern"},
		{fr: StacktraceFrame{AbsPath: path},
			libraryPattern:   regexp.MustCompile(""),
			libraryFrame:     newBool(true),
			origLibraryFrame: nil,
			msg:              "Empty pattern"},
		{fr: StacktraceFrame{LibraryFrame: newBool(false)},
			libraryPattern:   regexp.MustCompile("~"),
			libraryFrame:     newBool(false),
			origLibraryFrame: newBool(false),
			msg:              "Empty StacktraceFrame"},
		{fr: StacktraceFrame{AbsPath: path, LibraryFrame: newBool(true)},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     newBool(false),
			origLibraryFrame: newBool(true),
			msg:              "AbsPath given, no Match"},
		{fr: StacktraceFrame{Filename: "myFile.js", LibraryFrame: newBool(true)},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     newBool(false),
			origLibraryFrame: newBool(true),
			msg:              "Filename given, no Match"},
		{fr: StacktraceFrame{AbsPath: path, Filename: "myFile.js"},
			libraryPattern:   regexp.MustCompile("^~/"),
			libraryFrame:     newBool(false),
			origLibraryFrame: nil,
			msg:              "AbsPath and Filename given, no Match"},
		{fr: StacktraceFrame{Filename: "/tmp"},
			libraryPattern:   regexp.MustCompile("/tmp"),
			libraryFrame:     newBool(true),
			origLibraryFrame: nil,
			msg:              "Filename matching"},
		{fr: StacktraceFrame{AbsPath: path, LibraryFrame: newBool(false)},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     newBool(true),
			origLibraryFrame: newBool(false),
			msg:              "AbsPath matching"},
		{fr: StacktraceFrame{AbsPath: path, Filename: "/a/b/c"},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     newBool(true),
			origLibraryFrame: nil,
			msg:              "AbsPath matching, Filename not matching"},
		{fr: StacktraceFrame{AbsPath: path, Filename: "/a/b/c"},
			libraryPattern:   regexp.MustCompile("/a/b/c"),
			libraryFrame:     newBool(true),
			origLibraryFrame: nil,
			msg:              "AbsPath not matching, Filename matching"},
		{fr: StacktraceFrame{AbsPath: path, Filename: "~/a/b/c"},
			libraryPattern:   regexp.MustCompile("~/"),
			libraryFrame:     newBool(true),
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

func newBool(v bool) *bool {
	return &v
}
