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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearerAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		builder    bearerBuilder
		token      string
		authorized bool
	}{
		"empty":           {builder: bearerBuilder{}, authorized: true},
		"empty for token": {builder: bearerBuilder{}, authorized: false, token: "1"},
		"no token":        {builder: bearerBuilder{"123"}, authorized: false},
		"invalid token":   {builder: bearerBuilder{"123"}, authorized: false, token: "1"},
		"valid token":     {builder: bearerBuilder{"123"}, authorized: true, token: "123"},
	} {
		t.Run(name, func(t *testing.T) {
			bearer := tc.builder.forToken(tc.token)
			result, err := bearer.AuthorizedFor(context.Background(), "")
			assert.NoError(t, err)
			assert.Equal(t, Result{Authorized: tc.authorized}, result)
		})
	}
}
