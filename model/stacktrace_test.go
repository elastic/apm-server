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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
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
				LibraryFrame: true,
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
		output := test.Stacktrace.transform()
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
