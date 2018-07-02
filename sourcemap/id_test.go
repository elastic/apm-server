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

package sourcemap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmapIdKey(t *testing.T) {
	tests := []struct {
		id    Id
		key   string
		valid bool
	}{
		{
			id:    Id{ServiceName: "foo", ServiceVersion: "1.0", Path: "a/b"},
			key:   "foo_1.0_a/b",
			valid: true,
		},
		{
			id:    Id{ServiceName: "foo", ServiceVersion: "bar"},
			key:   "foo_bar",
			valid: false,
		},
		{id: Id{ServiceName: "foo"}, key: "foo", valid: false},
		{id: Id{ServiceVersion: "1"}, key: "1", valid: false},
		{id: Id{Path: "/tmp/a"}, key: "/tmp/a", valid: false},
	}
	for idx, test := range tests {
		assert.Equal(t, test.key, test.id.Key(),
			fmt.Sprintf("(%v) Expected Key() to return <%v> but received<%v>", idx, test.key, test.id.Key()))
		assert.Equal(t, test.valid, test.id.Valid(),
			fmt.Sprintf("(%v) Expected Valid() to be <%v>", idx, test.valid))
	}
}
