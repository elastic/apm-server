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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestAuthorizationMiddleware(t *testing.T) {

	for name, tc := range map[string]struct {
		header             string
		allowedWhenSecured bool
	}{
		"no header":      {},
		"invalid header": {header: "Foo Bar"},
		"invalid token":  {header: "Bearer Bar"},
		"bearer":         {header: "Bearer foo", allowedWhenSecured: true},
	} {
		setup := func(token string) (*authorization.Handler, *request.Context, *httptest.ResponseRecorder) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			if tc.header != "" {
				c.Request.Header.Set(headers.Authorization, tc.header)
			}
			builder, err := authorization.NewBuilder(&config.Config{SecretToken: token})
			require.NoError(t, err)
			return builder.ForAnyOfPrivileges(authorization.ActionAny), c, rec
		}

		t.Run(name+"secured apply", func(t *testing.T) {
			handler, c, rec := setup("foo")
			m := AuthorizationMiddleware(handler, true)
			Apply(m, beatertest.Handler202)(c)
			if tc.allowedWhenSecured {
				require.Equal(t, http.StatusAccepted, rec.Code)
			} else {
				require.Equal(t, http.StatusUnauthorized, rec.Code)
			}
		})

		t.Run(name+"secured", func(t *testing.T) {
			handler, c, rec := setup("foo")
			m := AuthorizationMiddleware(handler, false)
			Apply(m, beatertest.Handler202)(c)
			require.Equal(t, http.StatusAccepted, rec.Code)
		})

		t.Run(name+"unsecured apply", func(t *testing.T) {
			handler, c, rec := setup("")
			m := AuthorizationMiddleware(handler, true)
			Apply(m, beatertest.Handler202)(c)
			require.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, authorization.AllowAuth{}, c.Authorization)
		})

		t.Run(name+"unsecured", func(t *testing.T) {
			handler, c, rec := setup("")
			m := AuthorizationMiddleware(handler, false)
			Apply(m, beatertest.Handler202)(c)
			require.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, authorization.AllowAuth{}, c.Authorization)

		})
	}
}
