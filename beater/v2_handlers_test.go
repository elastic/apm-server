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

package beater

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNDJSONStreamReader(t *testing.T) {
	lines := []string{
		`{"key": "value1"}`,
		`{"key": "value2"}`,
		`{invalid-json}`,
		`{"key": "value3"}`,
	}
	expected := []struct {
		errPattern string
		out        map[string]interface{}
		isEOF      bool
	}{
		{
			out: map[string]interface{}{"key": "value1"},
		},
		{
			out: map[string]interface{}{"key": "value2"},
		},
		{
			out:        nil,
			errPattern: "invalid character",
		},
		{
			out:        map[string]interface{}{"key": "value3"},
			errPattern: "EOF",
			isEOF:      true,
		},
	}
	buf := bytes.NewBufferString(strings.Join(lines, "\n"))
	n := NDJSONStreamReader{stream: bufio.NewReader(buf)}

	for idx, test := range expected {
		out, err := n.Read()
		assert.Equal(t, test.out, out, "Failed at idx %v", idx)
		if test.errPattern == "" {
			assert.Nil(t, err)
		} else {
			assert.Contains(t, err.Error(), test.errPattern, "Failed at idx %v", idx)
		}
	}
}

func TestStreamResponse(t *testing.T) {
	sr := streamResponse{}
	transmogrifierErr := errors.New("transmogrifier error")
	err1 := cannotDecodeResponse(transmogrifierErr)
	sr.addError(err1)
	sr.addErrorCount(err1, 10)

	err2 := cannotValidateResponse(transmogrifierErr)
	sr.addError(err2)
	sr.addErrorCount(err2, 11)

	expected := streamResponse{
		Errors: map[int]map[string]uint{
			http.StatusBadRequest: map[string]uint{
				err1.err.Error(): 11,
				err2.err.Error(): 12,
			},
		},
	}

	assert.Equal(t, expected, sr)
}
