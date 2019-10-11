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

package authorization

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearer_AuthorizationRequired(t *testing.T) {
	handler := Bearer{}
	assert.True(t, handler.AuthorizationRequired())
}

func TestBearer_AuthorizedFor(t *testing.T) {
	for name, tc := range map[string]struct {
		handler    *Bearer
		authorized bool
	}{
		"no token":      {handler: &Bearer{}, authorized: false},
		"empty token":   {handler: NewBearer("", ""), authorized: true},
		"invalid token": {handler: NewBearer("abc", "abx"), authorized: false},
		"valid token":   {handler: NewBearer("foo", "foo"), authorized: true},
	} {
		t.Run(name, func(t *testing.T) {
			authorized, err := tc.handler.AuthorizedFor("", "")
			assert.NoError(t, err)
			assert.Equal(t, tc.authorized, authorized)
		})
	}
}
