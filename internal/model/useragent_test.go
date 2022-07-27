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

	"github.com/elastic/elastic-agent-libs/mapstr"
)

func TestUserAgentFields(t *testing.T) {
	tests := []struct {
		UserAgent UserAgent
		Output    mapstr.M
	}{{
		UserAgent: UserAgent{},
		Output:    nil,
	}, {
		UserAgent: UserAgent{Original: "rum-1.0"},
		Output:    mapstr.M{"original": "rum-1.0"},
	}, {
		UserAgent: UserAgent{Name: "mosaic"},
		Output:    mapstr.M{"name": "mosaic"},
	}}

	for _, test := range tests {
		output := test.UserAgent.fields()
		assert.Equal(t, test.Output, output)
	}
}
