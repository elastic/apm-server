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

package modeldecoder

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model"
)

func TestStacktraceFrameDecode(t *testing.T) {
	filename, classname, path, context, fct, module := "some file", "foo", "path", "contet", "fct", "module"
	lineno, colno := 1, 55
	libraryFrame := true
	vars := map[string]interface{}{"a": 1}
	pre_context, post_context := []string{"a"}, []string{"b", "c"}
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *model.StacktraceFrame
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), s: nil},
		{input: "", err: errInvalidStacktraceFrameType, s: nil},
		{
			input: map[string]interface{}{},
			s: &model.StacktraceFrame{
				AbsPath: nil, Lineno: nil, Colno: nil,
				ContextLine: nil, Module: nil, Function: nil, LibraryFrame: nil,
				Vars: nil, PreContext: nil, PostContext: nil},
		},
		{
			input: map[string]interface{}{
				"abs_path":      path,
				"filename":      filename,
				"classname":     classname,
				"lineno":        1.0,
				"colno":         55.0,
				"context_line":  context,
				"function":      fct,
				"module":        module,
				"library_frame": libraryFrame,
				"vars":          vars,
				"pre_context":   []interface{}{"a"},
				"post_context":  []interface{}{"b", "c"},
			},
			err: nil,
			s: &model.StacktraceFrame{
				AbsPath:      &path,
				Filename:     &filename,
				Classname:    &classname,
				Lineno:       &lineno,
				Colno:        &colno,
				ContextLine:  &context,
				Module:       &module,
				Function:     &fct,
				LibraryFrame: &libraryFrame,
				Vars:         vars,
				PreContext:   pre_context,
				PostContext:  post_context,
			},
		},
	} {
		frame, err := decodeStacktraceFrame(test.input, false, test.inpErr)
		assert.Equal(t, test.s, frame)
		assert.Equal(t, test.err, err)
	}
}
