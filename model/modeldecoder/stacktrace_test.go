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

func TestStacktraceDecode(t *testing.T) {
	l1 := 1
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		s           *model.Stacktrace
	}{
		{input: nil, err: nil, s: nil},
		{input: nil, inpErr: errors.New("msg"), err: errors.New("msg"), s: nil},
		{input: "", err: errors.New("invalid type for stacktrace"), s: nil},
		{
			input: []interface{}{"foo"},
			err:   errInvalidStacktraceFrameType,
			s:     &model.Stacktrace{nil},
		},
		{
			input: []interface{}{map[string]interface{}{"lineno": 1.0}},
			err:   nil,
			s: &model.Stacktrace{
				&model.StacktraceFrame{Lineno: &l1},
			},
		},
	} {
		s, err := decodeStacktrace(test.input, false, test.inpErr)
		assert.Equal(t, test.s, s)
		assert.Equal(t, test.err, err)
	}
}
