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
	"crypto/subtle"

	"github.com/elastic/apm-server/elasticsearch"
)

type bearerBuilder struct {
	required string
}

type bearerAuth struct {
	authorized bool
}

func (b bearerBuilder) forToken(token string) *bearerAuth {
	return &bearerAuth{
		authorized: subtle.ConstantTimeCompare([]byte(b.required), []byte(token)) == 1,
	}
}

func (b *bearerAuth) AuthorizedFor(context.Context, elasticsearch.Resource) (Result, error) {
	return Result{Authorized: b.authorized}, nil
}
