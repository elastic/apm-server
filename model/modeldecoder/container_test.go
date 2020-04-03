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

	"github.com/elastic/apm-server/model/metadata"
	"github.com/stretchr/testify/assert"
)

func TestContainerDecode(t *testing.T) {
	id := "container-id"
	for _, test := range []struct {
		input       interface{}
		err, inpErr error
		c           *metadata.Container
	}{
		{input: nil, err: nil, c: nil},
		{input: nil, inpErr: errors.New("a"), err: errors.New("a"), c: nil},
		{input: "", err: errors.New("invalid type for container"), c: nil},
		{
			input: map[string]interface{}{
				"id": id,
			},
			err: nil,
			c:   &metadata.Container{ID: id},
		},
	} {
		container, out := decodeContainer(test.input, test.inpErr)
		assert.Equal(t, test.c, container)
		assert.Equal(t, test.err, out)
	}
}
