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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/model/metadata"
)

func TestContainerDecode(t *testing.T) {
	id := "container-id"
	for _, test := range []struct {
		input map[string]interface{}
		c     metadata.Container
	}{
		{input: nil},
		{
			input: map[string]interface{}{"id": id},
			c:     metadata.Container{ID: id},
		},
	} {
		var container metadata.Container
		decodeContainer(test.input, &container)
		assert.Equal(t, test.c, container)
	}
}
