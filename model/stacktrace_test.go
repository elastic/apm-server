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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/common"
)

func TestStacktraceDecode(t *testing.T) {
	l1 := 1
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *Stacktrace
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("msg"), err: errors.New("msg"), s: nil},
		{input: "", err: errors.New("invalid type for stacktrace"), s: nil},
		{
			input: []interface{}{"foo"},
			err:   errors.New("invalid type for stacktrace frame"),
			s:     &Stacktrace{nil},
		},
		{
			input: []interface{}{map[string]interface{}{
				"filename": "file", "lineno": 1.0},
			},
			err: nil,
			s: &Stacktrace{
				&StacktraceFrame{Filename: "file", Lineno: &l1},
			},
		},
	} {
		s, err := DecodeStacktrace(test.input, test.inpErr)
		assert.Equal(t, test.s, s)
		assert.Equal(t, test.err, err)
	}
}

func TestStacktraceTransform(t *testing.T) {
	colno := 1
	l4, l5, l6, l8 := 4, 5, 6, 8
	fct := "original function"
	absPath, serviceName := "original path", "service1"
	service := metadata.Service{Name: &serviceName}

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
			Output: []common.MapStr{
				{
					"filename":              "",
					"exclude_from_grouping": false,
				},
			},
			Msg: "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: &l6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: &l8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l5,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: "/webpack",
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
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
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

	tctx := transform.Context{
		Metadata: metadata.Metadata{
			Service: &service,
		},
	}
	for idx, test := range tests {
		output := test.Stacktrace.Transform(&tctx)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}

func TestStacktraceTransformWithSourcemapping(t *testing.T) {
	colno := 1
	l4, l5, l6, l8 := 4, 5, 6, 8
	fct := "original function"
	absPath, serviceName := "original path", "service1"
	service := metadata.Service{Name: &serviceName}

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
			Output: []common.MapStr{
				{"filename": "",
					"exclude_from_grouping": false,
					"sourcemap": common.MapStr{
						"error":   "Colno mandatory for sourcemapping.",
						"updated": false,
					},
				},
			},
			Msg: "Stacktrace with empty Frame",
		},
		{
			Stacktrace: Stacktrace{&StacktraceFrame{Colno: &colno}},
			Output: []common.MapStr{
				{"filename": "",
					"line":                  common.MapStr{"column": 1},
					"exclude_from_grouping": false,
					"sourcemap": common.MapStr{
						"error":   "Lineno mandatory for sourcemapping.",
						"updated": false,
					},
				},
			},
			Msg: "Stacktrace with no lineno",
		},
		{
			Stacktrace: Stacktrace{
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{Colno: &colno, Lineno: &l6, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{Colno: &colno, Lineno: &l8, Function: &fct, AbsPath: &absPath},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l5,
					Filename: "original filename",
					Function: &fct,
					AbsPath:  &absPath,
				},
				&StacktraceFrame{
					Colno:    &colno,
					Lineno:   &l4,
					Filename: "/webpack",
					AbsPath:  &absPath,
				},
			},
			Output: []common.MapStr{
				{
					"abs_path": "changed path", "filename": "changed filename", "function": "<unknown>",
					"line":                  common.MapStr{"column": 100, "number": 400, "context": ""},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "original filename",
						"function": "original function",
						"lineno":   4,
					},
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 6},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": false, "error": "Some key error"},
				},
				{
					"abs_path": "original path", "filename": "", "function": "original function",
					"line":                  common.MapStr{"column": 1, "number": 8},
					"exclude_from_grouping": false,
				},
				{
					"abs_path": "changed path", "filename": "original filename", "function": "changed function",
					"line":                  common.MapStr{"column": 100, "number": 500, "context": ""},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "original filename",
						"function": "original function",
						"lineno":   5,
					},
				},
				{
					"abs_path": "changed path", "filename": "changed filename", "function": "<anonymous>",
					"line":                  common.MapStr{"column": 100, "number": 400, "context": ""},
					"exclude_from_grouping": false,
					"sourcemap":             common.MapStr{"updated": true},
					"original": common.MapStr{
						"abs_path": "original path",
						"colno":    1,
						"filename": "/webpack",
						"lineno":   4,
					},
				},
			},
			Msg: "Stacktrace with sourcemapping",
		},
	}

	for idx, test := range tests {
		tctx := &transform.Context{
			Config:   transform.Config{SmapMapper: &FakeMapper{}},
			Metadata: metadata.Metadata{Service: &service},
		}

		// run `Stacktrace.Transform` twice to ensure method is idempotent
		test.Stacktrace.Transform(tctx)
		output := test.Stacktrace.Transform(tctx)
		assert.Equal(t, test.Output, output, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
	}
}
