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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestContainerTransform(t *testing.T) {
	id := "container-id"

	tests := []struct {
		Container Container
		Output    common.MapStr
	}{
		{
			Container: Container{},
			Output:    nil,
		},
		{
			Container: Container{ID: id},
			Output:    common.MapStr{"id": id},
		},
		{
			Container: Container{Name: "container_name"},
			Output:    common.MapStr{"name": "container_name"},
		},
		{
			Container: Container{Runtime: "container_runtime"},
			Output:    common.MapStr{"runtime": "container_runtime"},
		},
		{
			Container: Container{ImageName: "image_name"},
			Output:    common.MapStr{"image": common.MapStr{"name": "image_name"}},
		},
		{
			Container: Container{ImageTag: "image_tag"},
			Output:    common.MapStr{"image": common.MapStr{"tag": "image_tag"}},
		},
	}

	for _, test := range tests {
		output := test.Container.fields()
		assert.Equal(t, test.Output, output)
	}
}
